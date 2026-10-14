package engine

import (
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// tracer follows error values backward through the module's SSA value flow,
// summarizing each function it meets by what it can return under which
// arguments.
type tracer struct {
	a      *analysis
	judged map[*types.Func]bool

	summaries map[*ssa.Function]*summary

	// epoch counts the passes over the judged functions. Summaries are recomputed
	// once per pass until one changes nothing, which settles calls that recurse.
	epoch   int
	changed bool

	// free traces values whose function's parameters are resolved through its
	// callers rather than left to one call: what a field or a variable is stored
	// with, and what flows into an unjudged function from its callers.
	free *frame

	// callbacks holds, per function, the receivers of the foreign calls
	// in it that are handed a module function value.
	callbacks map[*ssa.Function]map[receiverKey]bool
}

// summary is what a function can return.
type summary struct {
	epoch int

	// results are the origins each result can have, over every return.
	results []origins

	// returns are each return's origins, per result.
	returns []returned
}

type returned struct {
	ret     *ssa.Return
	results []origins
}

// frame is one function's tracing: its parameters and free variables are left
// for a call to resolve. A frame for no function resolves them through
// callers.
type frame struct {
	fn   *ssa.Function
	memo map[ssa.Value]origins

	// stack holds the depth each value being traced sits at. A value met again
	// while it is on the stack adds nothing, since a loop adds no origin it does
	// not already reach, and cut records the shallowest such value: a result
	// that depended on a value still being traced is incomplete, and only
	// the value at the head of the loop is memoized.
	stack map[ssa.Value]int
	cut   int
}

func newFrame(fn *ssa.Function) *frame {
	return &frame{fn: fn, memo: map[ssa.Value]origins{}, stack: map[ssa.Value]int{}, cut: math.MaxInt}
}

func newTracer(a *analysis, judged map[*types.Func]bool) *tracer {
	return &tracer{
		a:         a,
		judged:    judged,
		summaries: map[*ssa.Function]*summary{},
		callbacks: map[*ssa.Function]map[receiverKey]bool{},
	}
}

// maxPasses bounds the passes settle makes. Each pass recomputes every summary
// from the last pass's, and whether a wrapping is structured does not only
// grow with what its operands can be: an operand that could only be nil
// structures nothing until a structured origin reaches it. So passes are not
// guaranteed to settle, though in practice a few do.
const maxPasses = 64

// settle summarizes fns, and everything they reach, until a pass changes
// nothing, or maxPasses have run.
func (t *tracer) settle(fns []*ssa.Function) {
	for range maxPasses {
		t.epoch++
		t.changed = false
		t.free = newFrame(nil)
		for _, fn := range fns {
			t.summary(fn)
		}
		if !t.changed {
			return
		}
	}
}

func (t *tracer) summary(fn *ssa.Function) *summary {
	s := t.summaries[fn]
	if s == nil {
		s = &summary{results: make([]origins, fn.Signature.Results().Len())}
		t.summaries[fn] = s
	}
	if s.epoch == t.epoch {
		return s
	}
	s.epoch = t.epoch

	fr := newFrame(fn)
	results := make([]origins, len(s.results))
	var returns []returned
	for _, block := range fn.Blocks {
		ret, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return)
		if !ok {
			continue
		}
		each := make([]origins, len(results))
		for i, value := range ret.Results {
			if errorValued(fn.Signature.Results().At(i).Type()) {
				each[i] = t.trace(fr, value)
				results[i] = results[i].join(each[i])
			}
		}
		returns = append(returns, returned{ret: ret, results: each})
	}
	for i := range results {
		if !results[i].equal(s.results[i]) {
			t.changed = true
		}
	}
	s.results, s.returns = results, returns
	return s
}

func (t *tracer) trace(fr *frame, v ssa.Value) origins {
	if o, ok := fr.memo[v]; ok {
		return o
	}
	if depth, ok := fr.stack[v]; ok {
		fr.cut = min(fr.cut, depth)
		return origins{}
	}
	depth := len(fr.stack)
	fr.stack[v] = depth
	outer := fr.cut
	fr.cut = math.MaxInt

	o := t.origin(fr, v)

	delete(fr.stack, v)
	if fr.cut >= depth {
		fr.memo[v] = o
	}
	fr.cut = min(outer, fr.cut)
	if fr.cut >= depth {
		fr.cut = outer
	}
	return o
}

func (t *tracer) origin(fr *frame, v ssa.Value) origins {
	if typeParam(v.Type()) {
		return structuredOrigin()
	}
	switch v := v.(type) {
	case *ssa.Const:
		return origins{nilable: true}
	case *ssa.Parameter:
		return t.param(fr, v)
	case *ssa.FreeVar:
		return t.captured(fr, v)
	case *ssa.Phi:
		var out origins
		for _, edge := range v.Edges {
			out = out.join(t.trace(fr, edge))
		}
		return out
	case *ssa.MakeInterface:
		return t.concrete(v.X.Type(), v)
	case *ssa.ChangeInterface:
		return t.trace(fr, v.X)
	case *ssa.ChangeType:
		return t.trace(fr, v.X)
	case *ssa.Call:
		return t.call(fr, &v.Call, 0, v)
	case *ssa.Extract:
		return t.extract(fr, v)
	case *ssa.UnOp:
		switch v.Op {
		case token.MUL:
			return t.load(fr, v)
		case token.ARROW:
			return t.receive(v.X, v)
		}
	case *ssa.Field:
		return t.field(v.X.Type(), v.Field, v)
	}
	// Everything else — a type assertion, a load from a slice, map or pointer —
	// is not judged.
	return structuredOrigin()
}

func (t *tracer) extract(fr *frame, v *ssa.Extract) origins {
	switch tuple := v.Tuple.(type) {
	case *ssa.Call:
		return t.call(fr, &tuple.Call, v.Index, v)
	case *ssa.UnOp:
		if tuple.Op == token.ARROW && v.Index == 0 {
			return t.receive(tuple.X, v)
		}
	case *ssa.Select:
		// The tuple is the chosen index, whether a receive succeeded, then one
		// value per receiving state.
		received := v.Index - 2
		for _, state := range tuple.States {
			if state.Dir != types.RecvOnly {
				continue
			}
			if received == 0 {
				return t.receive(state.Chan, v)
			}
			received--
		}
	}
	return structuredOrigin()
}

func (t *tracer) witness(what string, at ssa.Value) *witness {
	where := topLevel(at.Parent())
	if origin := where.Origin(); origin != nil {
		where = origin
	}
	return &witness{what: what, where: where, position: t.a.module.Fset.PositionFor(at.Pos(), false)}
}

// placeName names the function a witness sits in. A wrapper the builder made
// for a method is named by the type it makes the method one of.
func placeName(fn *ssa.Function) string {
	if recv := fn.Signature.Recv(); recv != nil && synthesized(fn) {
		if named, ok := types.Unalias(pointee(recv.Type())).(*types.Named); ok {
			return named.Origin().Obj().Name() + "." + fn.Name()
		}
	}
	if obj := declaredFunc(fn); obj != nil {
		return funcName(obj)
	}
	name, _, _ := strings.Cut(fn.Name(), "#")
	return name
}

// placePackage is the package the function a witness sits in belongs to:
// for a wrapper the builder made for a method, the package of the type it makes
// the method one of.
func placePackage(fn *ssa.Function) *types.Package {
	if recv := fn.Signature.Recv(); recv != nil && synthesized(fn) {
		if named, ok := types.Unalias(pointee(recv.Type())).(*types.Named); ok {
			return named.Origin().Obj().Pkg()
		}
	}
	return home(fn)
}

// place names where a witness sits for a finding about a function of pkg,
// qualified by its own package's name when that is another.
func place(w *witness, pkg *types.Package) string {
	name := placeName(w.where)
	if w.initializes != "" {
		name = w.initializes
	}
	if home := placePackage(w.where); home != nil && home != pkg {
		name = home.Name() + "." + name
	}
	if w.initializes != "" {
		return "the initializer of " + name
	}
	return name
}

// declaredFunc returns the declaration of a module function, generic ones
// as their origin, or nil for a closure. A wrapper is its method's.
func declaredFunc(fn *ssa.Function) *types.Func {
	if fn.Parent() != nil {
		return nil
	}
	if origin := fn.Origin(); origin != nil {
		fn = origin
	}
	obj, ok := fn.Object().(*types.Func)
	if !ok {
		return nil
	}
	return obj.Origin()
}

func (t *tracer) param(fr *frame, p *ssa.Parameter) origins {
	fn := p.Parent()
	i := indexOf(fn.Params, p)
	if fn == fr.fn {
		return origins{deps: []dep{{index: i}}}
	}
	return t.callers(fn, i)
}

// callers resolves a parameter through every call passing it: the static calls
// of the function, and the calls through a closed module interface that reach
// it as a method. A judged function's parameter is its caller's own error,
// and one of a function whose callers are not all known is not judged: one put
// to other uses than calls, or a method an interface outside the module's sight
// dispatches to.
func (t *tracer) callers(fn *ssa.Function, i int) origins {
	if fn.Parent() != nil || synthesized(fn) || t.a.idx.escapes[fn] {
		return structuredOrigin()
	}
	obj := declaredFunc(fn)
	if obj != nil && t.judged[obj] {
		return structuredOrigin()
	}
	var out origins
	called := false
	for _, site := range t.a.idx.calls[fn] {
		if i < len(site.call.Args) {
			called = true
			out = out.join(t.trace(t.free, site.call.Args[i]))
		}
	}
	if obj == nil {
		return out
	}
	for _, answer := range t.a.idx.dispatch[obj] {
		named, ok := types.Unalias(answer.iface).(*types.Named)
		if !ok || !t.a.module.Declares(named.Obj()) || !closed(named) {
			return structuredOrigin()
		}
		for _, site := range t.a.idx.invokes[answer.method] {
			// An interface call passes no receiver; the method's is its first parameter.
			if !types.Identical(site.call.Value.Type(), answer.iface) || i == 0 || i > len(site.call.Args) {
				continue
			}
			called = true
			out = out.join(t.trace(t.free, site.call.Args[i-1]))
		}
	}
	if !called {
		return structuredOrigin()
	}
	return out
}

// captured is a free variable of a closure, or a load from one: what
// the closure's binding for it holds. Inside the closure's own frame it is left
// for the call making the closure to resolve; anywhere else it is what every
// binding of it holds.
func (t *tracer) captured(fr *frame, fv *ssa.FreeVar) origins {
	fn := fv.Parent()
	i := indexOf(fn.FreeVars, fv)
	if fn == fr.fn {
		return origins{deps: []dep{{free: true, index: i}}}
	}
	var out origins
	for _, closure := range t.a.idx.closures[fn] {
		out = out.join(t.binding(fr, closure.Bindings[i]))
	}
	return out
}

// binding is what a closure's binding of a free variable holds: a captured
// local's every store, a captured free variable of an enclosing closure,
// or a bound method's receiver.
func (t *tracer) binding(fr *frame, v ssa.Value) origins {
	switch v := v.(type) {
	case *ssa.Alloc:
		return t.cell(fr, v)
	case *ssa.FreeVar:
		return t.captured(fr, v)
	default:
		return t.trace(fr, v)
	}
}

func indexOf[T comparable](values []T, v T) int {
	for i, candidate := range values {
		if candidate == v {
			return i
		}
	}
	return -1
}

// concrete judges a value of a concrete type converted to an interface.
func (t *tracer) concrete(typ types.Type, at ssa.Value) origins {
	if typeParam(typ) {
		return structuredOrigin()
	}
	named := errorNamed(typ)
	if named == nil {
		return origins{produced: t.witness("a value of type "+types.TypeString(typ, packageName), at)}
	}
	if t.a.nameable(named) {
		return structuredOrigin()
	}
	return origins{produced: t.witness("a value of type "+typeName(named), at)}
}

func packageName(pkg *types.Package) string {
	return pkg.Name()
}

func (t *tracer) load(fr *frame, load *ssa.UnOp) origins {
	switch addr := load.X.(type) {
	case *ssa.Global:
		return t.global(addr, load)
	case *ssa.Alloc:
		return t.local(fr, load, addr)
	case *ssa.FreeVar:
		return t.captured(fr, addr)
	case *ssa.FieldAddr:
		return t.field(addr.X.Type(), addr.Field, load)
	default:
		return structuredOrigin()
	}
}

// local is a load of a local the function took the address of: what the stores
// reaching the load put there, and, when a closure capturing the local can run
// between those stores and the load, what the closure stores.
func (t *tracer) local(fr *frame, load *ssa.UnOp, alloc *ssa.Alloc) origins {
	if escapes(alloc) {
		return structuredOrigin()
	}
	reach := reaching(load, alloc)
	if reach.unknown {
		return t.cell(fr, alloc)
	}
	out := origins{nilable: reach.zero}
	for _, value := range reach.values {
		out = out.join(t.trace(fr, value))
	}
	if reach.interleaved || launched(alloc) {
		for _, value := range capturedStores(alloc) {
			out = out.join(t.trace(fr, value))
		}
	}
	return out
}

// cell is a captured local as a closure sees it, which can run after any
// store: every store to it.
func (t *tracer) cell(fr *frame, alloc *ssa.Alloc) origins {
	if escapes(alloc) {
		return structuredOrigin()
	}
	var out origins
	for _, ref := range *alloc.Referrers() {
		if store, ok := ref.(*ssa.Store); ok && store.Addr == alloc {
			out = out.join(t.trace(fr, store.Val))
		}
	}
	for _, value := range capturedStores(alloc) {
		out = out.join(t.trace(fr, value))
	}
	return out
}

// escapes reports whether a local's address is put to a use through which
// it may be written unseen.
func escapes(alloc *ssa.Alloc) bool {
	for _, ref := range *alloc.Referrers() {
		switch ref := ref.(type) {
		case *ssa.Store:
			if ref.Addr != alloc {
				return true
			}
		case *ssa.UnOp:
			if ref.Op != token.MUL {
				return true
			}
		case *ssa.MakeClosure, *ssa.DebugRef:
		default:
			return true
		}
	}
	return false
}

// launched reports whether a closure capturing a local runs in a goroutine
// of its own, which can store into the local at any time.
func launched(alloc *ssa.Alloc) bool {
	for _, ref := range *alloc.Referrers() {
		closure, ok := ref.(*ssa.MakeClosure)
		if !ok {
			continue
		}
		for _, use := range *closure.Referrers() {
			if goes, ok := use.(*ssa.Go); ok && goes.Call.Value == closure {
				return true
			}
		}
	}
	return false
}

// capturedStores returns what closures capturing a local store into it,
// through any depth of nesting.
func capturedStores(addr ssa.Value) []ssa.Value {
	var values []ssa.Value
	for _, ref := range *addr.Referrers() {
		closure, ok := ref.(*ssa.MakeClosure)
		if !ok {
			continue
		}
		fn, ok := closure.Fn.(*ssa.Function)
		if !ok {
			continue
		}
		for i, binding := range closure.Bindings {
			if binding != addr {
				continue
			}
			fv := fn.FreeVars[i]
			for _, use := range *fv.Referrers() {
				if store, ok := use.(*ssa.Store); ok && store.Addr == fv {
					values = append(values, store.Val)
				}
			}
			values = append(values, capturedStores(fv)...)
		}
	}
	return values
}

// reach is what a load from a local can read.
type reach struct {
	// values are what the stores reaching the load put in the local.
	values []ssa.Value

	// zero reports that a path reaches the local's allocation with no store,
	// where it holds its zero value, nil.
	zero bool

	// interleaved reports that, along some path, a call or the deferred calls
	// run between the load and the store or allocation reaching it: a closure
	// capturing the local may have stored into it there.
	interleaved bool

	// unknown reports that some path starts somewhere the function's flow does
	// not show, as the return a recovered panic resumes at does: any store
	// may reach the load.
	unknown bool
}

func reaching(load *ssa.UnOp, alloc *ssa.Alloc) reach {
	var r reach
	type visit struct {
		block  *ssa.BasicBlock
		called bool
	}
	seen := map[visit]bool{}
	entry := load.Parent().Blocks[0]
	var walk func(block *ssa.BasicBlock, upto int, called bool)
	walk = func(block *ssa.BasicBlock, upto int, called bool) {
		for _, instr := range slices.Backward(block.Instrs[:upto]) {
			switch instr := instr.(type) {
			case *ssa.Store:
				if instr.Addr == alloc {
					r.values = append(r.values, instr.Val)
					r.interleaved = r.interleaved || called
					return
				}
			case *ssa.Alloc:
				if instr == alloc {
					r.zero = true
					r.interleaved = r.interleaved || called
					return
				}
			case *ssa.Call, *ssa.Go, *ssa.RunDefers:
				called = true
			}
		}
		if len(block.Preds) == 0 && block != entry {
			r.unknown = true
		}
		for _, pred := range block.Preds {
			if v := (visit{pred, called}); !seen[v] {
				seen[v] = true
				walk(pred, len(pred.Instrs), called)
			}
		}
	}
	block := load.Block()
	walk(block, indexOf(block.Instrs, ssa.Instruction(load)), false)
	return r
}

// global judges a load of a package-level variable. A module variable counts
// as what it is stored with, unless it is a sentinel the configuration counts
// as structured.
func (t *tracer) global(g *ssa.Global, at ssa.Value) origins {
	obj := g.Object()
	if obj == nil {
		return structuredOrigin()
	}
	pkg := obj.Pkg()
	if t.a.module.Package(pkg) == nil {
		if t.a.trusted(pkg) {
			return structuredOrigin()
		}
		return origins{produced: t.witness(foreignName(obj), at)}
	}
	if t.a.cfg.Sentinels && obj.Exported() && t.a.api[pkg.Path()] {
		return structuredOrigin()
	}
	stores := t.a.idx.globalStores[g]
	out := origins{nilable: len(stores) == 0}
	for _, store := range stores {
		out = out.join(t.trace(t.free, store.value))
	}
	if w := out.produced; w != nil && w.where.Synthetic == "package initializer" {
		initialized := *w
		initialized.initializes = obj.Name()
		out.produced = &initialized
	}
	return out
}

// field judges a load of an error field of a struct of type holder: a module
// field counts as every store to it, and a foreign one as unstructured unless
// trusted.
func (t *tracer) field(holder types.Type, i int, at ssa.Value) origins {
	field := fieldOf(holder, i)
	if !errorValued(field.Type()) {
		return structuredOrigin()
	}
	if t.a.module.Declares(field) {
		return t.moduleField(holder, field)
	}
	if t.a.trusted(field.Pkg()) {
		return structuredOrigin()
	}
	return origins{produced: t.witness(fieldName(holder, field), at)}
}

// moduleField judges a load of a module struct's error field. A cause slot,
// an error field of a module error type, is not followed: whatever was stored
// there was judged where it was made, by the function returning it,
// and the stores of every value of the type together say nothing of the one
// loaded. What it holds is neutral. Any other field is every store to it,
// or nil, a field's zero value.
func (t *tracer) moduleField(holder types.Type, field *types.Var) origins {
	if named := errorNamed(holder); named != nil && t.a.module.Declares(named.Obj()) {
		return origins{neutral: true}
	}
	out := origins{nilable: true}
	for _, store := range t.a.idx.fieldStores[field] {
		out = out.join(t.trace(t.free, store.value))
	}
	return out
}

// fieldName names a field by the struct type holding it.
func fieldName(holder types.Type, field *types.Var) string {
	if named, ok := types.Unalias(pointee(holder)).(*types.Named); ok {
		return typeName(named) + "." + field.Name()
	}
	return foreignName(field)
}

// receive judges a receive from a channel. One foreign code provides, as one
// of its fields or a call's result, is unstructured unless trusted; any other
// receive is not judged.
func (t *tracer) receive(channel ssa.Value, at ssa.Value) origins {
	for {
		change, ok := channel.(*ssa.ChangeType)
		if !ok {
			break
		}
		channel = change.X
	}
	provider := ""
	trusted := false
	switch c := channel.(type) {
	case *ssa.UnOp:
		if addr, ok := c.X.(*ssa.FieldAddr); ok && c.Op == token.MUL {
			field := fieldOf(addr.X.Type(), addr.Field)
			if !t.a.module.Declares(field) {
				provider, trusted = fieldName(addr.X.Type(), field), t.a.trusted(field.Pkg())
			}
		}
	case *ssa.Field:
		field := fieldOf(c.X.Type(), c.Field)
		if !t.a.module.Declares(field) {
			provider, trusted = fieldName(c.X.Type(), field), t.a.trusted(field.Pkg())
		}
	case *ssa.Call:
		provider, trusted = t.foreignProvider(&c.Call)
	case *ssa.Extract:
		if call, ok := c.Tuple.(*ssa.Call); ok {
			provider, trusted = t.foreignProvider(&call.Call)
		}
	}
	if provider == "" || trusted {
		return structuredOrigin()
	}
	return origins{produced: t.witness("a receive from "+provider, at)}
}

// foreignProvider names a foreign function or interface method a call
// reaches, and whether it is trusted, or returns "" for a module call.
func (t *tracer) foreignProvider(call *ssa.CallCommon) (string, bool) {
	if call.IsInvoke() {
		named, ok := types.Unalias(call.Value.Type()).(*types.Named)
		if !ok || named.Obj().Pkg() == nil || t.a.module.Package(named.Obj().Pkg()) != nil {
			return "", false
		}
		return typeName(named) + "." + call.Method.Name(), t.a.trusted(named.Obj().Pkg())
	}
	callee := call.StaticCallee()
	if callee == nil || len(callee.Blocks) > 0 || callee.Object() == nil {
		return "", false
	}
	return foreignName(callee.Object()), t.a.trusted(callee.Object().Pkg())
}

// call judges one result of a call.
func (t *tracer) call(fr *frame, call *ssa.CallCommon, index int, at ssa.Value) origins {
	if call.IsInvoke() {
		return t.invoke(fr, call, index, at)
	}
	callee := call.StaticCallee()
	if callee == nil {
		return structuredOrigin()
	}
	if len(callee.Blocks) > 0 {
		return t.follow(fr, call, callee, index, false)
	}
	if t.handsCallback(at.Parent(), call) {
		return structuredOrigin()
	}
	return t.foreignCall(fr, call, callee, at)
}

func (t *tracer) foreignCall(fr *frame, call *ssa.CallCommon, callee *ssa.Function, at ssa.Value) origins {
	obj, ok := callee.Object().(*types.Func)
	if !ok {
		return structuredOrigin()
	}
	if obj.Signature().Recv() == nil && obj.Pkg() != nil {
		switch obj.Pkg().Path() + "." + obj.Name() {
		case "errors.New":
			return origins{produced: t.witness("errors.New", at)}
		case "fmt.Errorf":
			return t.errorf(fr, call, at)
		case "errors.Join":
			return t.join(fr, call, at)
		}
	}
	if t.a.trusted(obj.Pkg()) {
		return structuredOrigin()
	}
	return origins{produced: t.witness(foreignName(obj), at)}
}

// errorf judges fmt.Errorf through its %w operands, and does not judge one
// whose format is not a constant, since its verbs cannot be read.
func (t *tracer) errorf(fr *frame, call *ssa.CallCommon, at ssa.Value) origins {
	format, ok := call.Args[0].(*ssa.Const)
	if !ok || format.Value == nil || format.Value.Kind() != constant.String {
		return structuredOrigin()
	}
	args, ok := wholemodule.VariadicArgs(call)
	if !ok {
		return structuredOrigin()
	}
	wrapped, ok := wrapVerbs(constant.StringVal(format.Value))
	if !ok {
		return structuredOrigin()
	}
	var operands []origins
	for _, i := range wrapped {
		if i >= len(args) || args[i] == nil {
			return structuredOrigin()
		}
		operands = append(operands, t.operand(fr, args[i]))
	}
	return wrapping(operands, t.witness("fmt.Errorf", at), false)
}

// join judges errors.Join through its operands.
func (t *tracer) join(fr *frame, call *ssa.CallCommon, at ssa.Value) origins {
	args, ok := wholemodule.VariadicArgs(call)
	if !ok {
		return structuredOrigin()
	}
	var operands []origins
	for _, arg := range args {
		if arg != nil {
			operands = append(operands, t.operand(fr, arg))
		}
	}
	return wrapping(operands, t.witness("errors.Join", at), true)
}

// operand judges a value a variadic call was handed, as the call wrote it.
func (t *tracer) operand(fr *frame, arg ssa.Value) origins {
	switch arg := arg.(type) {
	case *ssa.MakeInterface:
		return t.concrete(arg.X.Type(), arg)
	case *ssa.ChangeInterface:
		return t.trace(fr, arg.X)
	default:
		return t.trace(fr, arg)
	}
}

func nilConst(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	return ok && c.IsNil()
}

// wrapVerbs returns the operand index each %w verb of a format consumes,
// or false when the format's operand indexing cannot be read.
func wrapVerbs(format string) ([]int, bool) {
	var wrapped []int
	operand := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		i++
		for i < len(format) && strings.IndexByte("+-# 0", format[i]) >= 0 {
			i++
		}
		var ok bool
		if i, operand, ok = argIndex(format, i, operand); !ok {
			return nil, false
		}
		i, operand = skipNumber(format, i, operand)
		if i < len(format) && format[i] == '.' {
			i++
			if i, operand, ok = argIndex(format, i, operand); !ok {
				return nil, false
			}
			i, operand = skipNumber(format, i, operand)
		}
		if i, operand, ok = argIndex(format, i, operand); !ok {
			return nil, false
		}
		if i >= len(format) {
			break
		}
		if format[i] == '%' {
			continue
		}
		if format[i] == 'w' {
			wrapped = append(wrapped, operand)
		}
		operand++
	}
	return wrapped, true
}

// argIndex reads an explicit [n] operand index at i.
func argIndex(format string, i, operand int) (int, int, bool) {
	if i >= len(format) || format[i] != '[' {
		return i, operand, true
	}
	end := strings.IndexByte(format[i:], ']')
	if end < 0 {
		return i, operand, false
	}
	n := 0
	for _, r := range format[i+1 : i+end] {
		if r < '0' || r > '9' {
			return i, operand, false
		}
		n = n*10 + int(r-'0')
	}
	if n < 1 {
		return i, operand, false
	}
	return i + end + 1, n - 1, true
}

// skipNumber skips a width or precision, a * consuming an operand.
func skipNumber(format string, i, operand int) (int, int) {
	if i < len(format) && format[i] == '*' {
		return i + 1, operand + 1
	}
	for i < len(format) && format[i] >= '0' && format[i] <= '9' {
		i++
	}
	return i, operand
}

// handsCallback reports whether a foreign call is handed a module function
// value, or is made on the receiver of such a call in the same function. Either
// returns what the callback returned, such as filepath.WalkDir
// and errgroup.Group.Wait, so neither is judged.
func (t *tracer) handsCallback(caller *ssa.Function, call *ssa.CallCommon) bool {
	if callback(call) {
		return true
	}
	recv, ok := receiver(call)
	if !ok {
		return false
	}
	receivers, ok := t.callbacks[caller]
	if !ok {
		receivers = map[receiverKey]bool{}
		for _, block := range caller.Blocks {
			for _, instr := range block.Instrs {
				if c, ok := instr.(ssa.CallInstruction); ok && t.foreign(c.Common()) && callback(c.Common()) {
					if r, ok := receiver(c.Common()); ok {
						receivers[r] = true
					}
				}
			}
		}
		t.callbacks[caller] = receivers
	}
	return receivers[recv]
}

// foreign reports whether a call reaches foreign code.
func (t *tracer) foreign(call *ssa.CallCommon) bool {
	if call.IsInvoke() {
		named, ok := types.Unalias(call.Value.Type()).(*types.Named)
		return ok && named.Obj().Pkg() != nil && t.a.module.Package(named.Obj().Pkg()) == nil
	}
	callee := call.StaticCallee()
	return callee != nil && len(callee.Blocks) == 0
}

// callback reports whether a call is handed a function value other than
// a foreign function.
func callback(call *ssa.CallCommon) bool {
	for _, arg := range call.Args {
		if _, ok := arg.Type().Underlying().(*types.Signature); !ok || nilConst(arg) {
			continue
		}
		for {
			change, ok := arg.(*ssa.ChangeType)
			if !ok {
				break
			}
			arg = change.X
		}
		if fn, ok := arg.(*ssa.Function); ok && len(fn.Blocks) == 0 {
			continue
		}
		return true
	}
	return false
}

// receiverKey identifies the variable a method call is made on: the value
// at the root of the loads and field selections reaching the receiver,
// and the fields selected from it. Two calls on g.group, each selecting
// the field afresh, share one.
type receiverKey struct {
	root ssa.Value

	// fields are the selected fields' indices, innermost first.
	fields string
}

// receiver returns the variable a method call is made on, or false
// for a function call.
func receiver(call *ssa.CallCommon) (receiverKey, bool) {
	var recv ssa.Value
	switch {
	case call.IsInvoke():
		recv = call.Value
	case call.Signature().Recv() != nil && len(call.Args) > 0:
		recv = call.Args[0]
	default:
		return receiverKey{}, false
	}
	var fields []string
	for {
		switch v := recv.(type) {
		case *ssa.UnOp:
			if v.Op != token.MUL {
				return receiverKey{root: recv, fields: strings.Join(fields, ".")}, true
			}
			recv = v.X
			continue
		case *ssa.FieldAddr:
			fields = append(fields, strconv.Itoa(v.Field))
			recv = v.X
			continue
		case *ssa.Field:
			fields = append(fields, strconv.Itoa(v.Field))
			recv = v.X
			continue
		}
		return receiverKey{root: recv, fields: strings.Join(fields, ".")}, true
	}
}

// invoke judges a call through an interface method: a foreign interface's
// counts as unstructured unless trusted, a closed module interface's
// as the union over its implementations, and an open one's is not judged.
func (t *tracer) invoke(fr *frame, call *ssa.CallCommon, index int, at ssa.Value) origins {
	named, ok := types.Unalias(call.Value.Type()).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return structuredOrigin()
	}
	pkg := named.Obj().Pkg()
	if t.a.module.Package(pkg) == nil {
		if t.handsCallback(at.Parent(), call) || t.a.trusted(pkg) {
			return structuredOrigin()
		}
		return origins{produced: t.witness(typeName(named)+"."+call.Method.Name(), at)}
	}
	if !closed(named) {
		return structuredOrigin()
	}
	var out origins
	for _, concrete := range t.a.idx.implementations(named) {
		sel := t.a.prog.MethodSets.MethodSet(concrete).Lookup(call.Method.Pkg(), call.Method.Name())
		if sel == nil {
			continue
		}
		impl := methodValue(t.a.prog.Program, sel)
		switch {
		case impl == nil:
		case len(impl.Blocks) > 0:
			out = out.join(t.follow(fr, call, impl, index, true))
		case !t.a.trusted(impl.Object().Pkg()):
			out = out.join(origins{produced: t.witness(foreignName(impl.Object()), at)})
		}
	}
	return out
}

// follow judges a call to a function with a body through its summary,
// resolving what depends on the call's arguments in the caller. Whatever
// is unstructured in a judged function's own results is that function's
// finding.
func (t *tracer) follow(fr *frame, call *ssa.CallCommon, callee *ssa.Function, index int, invoked bool) origins {
	s := t.summary(callee)
	if index >= len(s.results) {
		return structuredOrigin()
	}
	result := s.results[index]
	out := origins{structured: result.structured, nilable: result.nilable, neutral: result.neutral}
	if t.reports(callee, index) {
		out.viaJudged = true
	} else {
		out.produced, out.viaJudged = result.produced, result.viaJudged
	}

	resolved := map[dep]origins{}
	arg := func(d dep) origins {
		if o, ok := resolved[d]; ok {
			return o
		}
		o := structuredOrigin()
		switch {
		case d.free:
			if closure, ok := call.Value.(*ssa.MakeClosure); ok && d.index < len(closure.Bindings) {
				o = t.binding(fr, closure.Bindings[d.index])
			}
		default:
			// A method called through an interface takes its receiver from the call's
			// value, which is the interface rather than an error.
			i := d.index
			if invoked {
				i--
			}
			if i >= 0 && i < len(call.Args) {
				o = t.trace(fr, call.Args[i])
			}
		}
		resolved[d] = o
		return o
	}
	for _, d := range result.deps {
		out = out.join(arg(d))
	}
	for _, w := range result.wraps {
		out = out.join(resolve(w, arg))
	}
	return out
}

// reports reports whether a call's callee is a judged function whose own
// finding covers what the call's result can be unstructured by itself.
// An instance of a generic judged function, or a wrapper of a judged method,
// is covered by what the declaration reports, which is read from its own body:
// a value an instance's type arguments make unstructured is no finding
// of the declaration's, and stays with the caller.
func (t *tracer) reports(callee *ssa.Function, index int) bool {
	obj := declaredFunc(callee)
	if obj == nil || !t.judged[obj] {
		return false
	}
	body := t.a.idx.declared[obj]
	if body == nil {
		return false
	}
	results := t.summary(body).results
	return index < len(results) && results[index].reported()
}

// resolve settles a callee's wrapping against a call's arguments: rescued
// when every dep of one of its rescues structures a wrapping, conditional
// on the caller's own deps when that is what they depend on, and otherwise
// unstructured.
func resolve(w wrap, arg func(dep) origins) origins {
	var rescues []conjunction
	for _, rescue := range w.rescues {
		alternatives := []conjunction{{}}
		held := true
		for _, d := range rescue {
			conditions, ok := arg(d).condition()
			if ok {
				alternatives, ok = cross(alternatives, conditions)
			}
			if !ok {
				held = false
				break
			}
		}
		if !held {
			continue
		}
		if slices.ContainsFunc(alternatives, func(c conjunction) bool { return len(c) == 0 }) {
			return structuredOrigin()
		}
		rescues = append(rescues, alternatives...)
	}
	if len(rescues) == 0 {
		return unconditional(w.call, w.judged)
	}
	return origins{wraps: []wrap{{call: w.call, judged: w.judged, rescues: normalize(rescues)}}}
}
