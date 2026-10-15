package engine

import (
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/zcayou/go-tools/golangci/internal/enum"
)

// node is one place a flow decides where values come from: a struct field,
// a parameter, or a function's result. Exactly one of field, param and fn
// is set.
type node struct {
	field *types.Var
	param *ssa.Parameter
	fn    *ssa.Function
	index int
}

// origin is what a flow counts as the source of a value, as opposed to a value
// it cannot account for.
type origin interface {
	// constants reports whether a constant is a source, and so whether
	// a concatenation and a []string literal are followed to their parts.
	constants() bool

	// instances reports whether a store or call written in a generic body
	// is left to the body's instances, where the type arguments are known.
	// Otherwise the generic body's own values are judged, which hold whatever
	// a type argument makes them.
	instances() bool

	// result reports whether a static call's result is a source, and the type
	// whose String it renders, or nil when the origin tracks none.
	result(callee *ssa.Function) (types.Type, bool)
}

// flow decides which nodes only ever hold values from an origin.
// It is the greatest fixed point: every node starts out holding only those,
// and a node fed anything else loses it, as does every node depending on one
// that lost it, until nothing changes.
type flow struct {
	a      *analysis
	origin origin

	// deps are the nodes each node's values come from.
	deps map[node][]node

	// dynamic holds the nodes fed some value that comes from no origin whatever
	// the other nodes hold.
	dynamic map[node]bool

	// renders are the types whose String each node's own values are.
	renders map[node][]types.Type
}

func newFlow(a *analysis, o origin) *flow {
	return &flow{
		a:       a,
		origin:  o,
		deps:    map[node][]node{},
		dynamic: map[node]bool{},
		renders: map[node][]types.Type{},
	}
}

// stored reports whether some module code stores to a field.
func (f *flow) stored(field *types.Var) bool {
	return len(f.a.idx.fieldStores[field]) > 0
}

// build records the dependencies of a node and of every node it reaches.
func (f *flow) build(start node) {
	pending := []node{start}
	for len(pending) > 0 {
		n := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if _, built := f.deps[n]; built || f.dynamic[n] {
			continue
		}
		deps, ok := f.sources(n)
		f.deps[n] = deps
		if !ok {
			f.dynamic[n] = true
			continue
		}
		pending = append(pending, deps...)
	}
}

// settle returns the nodes that lose their origin.
func (f *flow) settle() map[node]bool {
	dependents := map[node][]node{}
	for n, deps := range f.deps {
		for _, dep := range deps {
			dependents[dep] = append(dependents[dep], n)
		}
	}
	lost := map[node]bool{}
	var pending []node
	for n := range f.dynamic {
		lost[n] = true
		pending = append(pending, n)
	}
	for len(pending) > 0 {
		n := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, dependent := range dependents[n] {
			if !lost[dependent] {
				lost[dependent] = true
				pending = append(pending, dependent)
			}
		}
	}
	return lost
}

// rendered returns the distinct types whose String a node holds, through every
// node it depends on.
func (f *flow) rendered(start node) []types.Type {
	var found []types.Type
	seen := map[node]bool{start: true}
	pending := []node{start}
	for len(pending) > 0 {
		n := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, t := range f.renders[n] {
			if !slices.ContainsFunc(found, func(other types.Type) bool { return types.Identical(t, other) }) {
				found = append(found, t)
			}
		}
		for _, dep := range f.deps[n] {
			if !seen[dep] {
				seen[dep] = true
				pending = append(pending, dep)
			}
		}
	}
	return found
}

// sources returns the nodes a node's values depend on, and false when one
// of them comes from no origin whatever the nodes hold.
func (f *flow) sources(n node) ([]node, bool) {
	var values []ssa.Value
	switch {
	case n.field != nil:
		if f.a.idx.unseen[n.field] {
			return nil, false
		}
		for _, store := range slices.Concat(f.a.idx.fieldStores[n.field], f.a.idx.elementStores[n.field]) {
			if !f.left(store.fn) {
				values = append(values, store.value)
			}
		}
	case n.param != nil:
		fn := n.param.Parent()
		if !f.followedParams(fn) {
			return nil, false
		}
		i := indexOf(fn.Params, n.param)
		for _, site := range f.a.idx.calls[fn] {
			if !f.left(site.caller) {
				values = append(values, site.call.Args[i])
			}
		}
	default:
		for _, block := range n.fn.Blocks {
			if ret, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return); ok {
				values = append(values, ret.Results[n.index])
			}
		}
	}

	t := &trail{visiting: map[ssa.Value]bool{}}
	deps, ok := f.values(t, values...)
	if !ok {
		return nil, false
	}
	f.renders[n] = t.renders
	return deps, true
}

// left reports whether a value written in fn is left to fn's instances.
func (f *flow) left(fn *ssa.Function) bool {
	return f.origin.instances() && parameterized(fn)
}

// trail is one walk back from a node's values: the values it has visited,
// and the types whose String the values it reached are.
type trail struct {
	visiting map[ssa.Value]bool
	renders  []types.Type
}

// followedParams reports whether a function's parameters are followed to its
// callers: an unexported module function, called only statically, that some
// module code calls. A generic body's parameters are not followed: what
// they hold depends on its type arguments.
func (f *flow) followedParams(fn *ssa.Function) bool {
	obj := declaredFunc(fn)
	if obj == nil || obj.Exported() || synthesized(fn) || parameterized(fn) || !f.a.module.Declares(obj) {
		return false
	}
	return !f.a.idx.escapes[fn] && len(f.a.idx.dispatch[obj]) == 0 && len(f.a.idx.calls[fn]) > 0
}

// followedResults reports whether a function's results are followed:
// an unexported module function with a body.
func (f *flow) followedResults(fn *ssa.Function) bool {
	obj := declaredFunc(fn)
	return obj != nil && !obj.Exported() && f.a.module.Declares(obj) && len(fn.Blocks) > 0 && !parameterized(fn) &&
		!synthesized(fn)
}

// value returns the nodes a value comes from, and false when it comes from
// no origin whatever they hold.
func (f *flow) value(v ssa.Value, t *trail) ([]node, bool) {
	if t.visiting[v] {
		return nil, true
	}
	t.visiting[v] = true
	switch v := v.(type) {
	case *ssa.Const:
		return nil, f.origin.constants()
	case *ssa.BinOp:
		if v.Op != token.ADD || !f.origin.constants() {
			return nil, false
		}
		return f.values(t, v.X, v.Y)
	case *ssa.Phi:
		return f.values(t, v.Edges...)
	case *ssa.UnOp:
		if v.Op != token.MUL {
			return nil, false
		}
		return f.load(v.X, t)
	case *ssa.Field:
		return f.fieldNode(fieldOf(v.X.Type(), v.Field))
	case *ssa.Slice:
		if !f.origin.constants() {
			return nil, false
		}
		return f.sliceLiteral(v, t)
	case *ssa.Parameter:
		if !f.followedParams(v.Parent()) {
			return nil, false
		}
		return []node{{param: v}}, true
	case *ssa.Call:
		return f.result(&v.Call, 0, t)
	case *ssa.Extract:
		if call, ok := v.Tuple.(*ssa.Call); ok {
			return f.result(&call.Call, v.Index, t)
		}
	}
	return nil, false
}

func (f *flow) values(t *trail, values ...ssa.Value) ([]node, bool) {
	var deps []node
	for _, value := range values {
		more, ok := f.value(value, t)
		if !ok {
			return nil, false
		}
		deps = append(deps, more...)
	}
	return deps, true
}

// fieldNode follows a load from a field to the field's stores. A consumer can
// store into an exported field of an API package, so its stores are not all
// visible, just as an exported function's calls are not.
func (f *flow) fieldNode(field *types.Var) ([]node, bool) {
	if !f.a.module.Declares(field) || !textual(field.Type()) || field.Exported() && f.a.api[field.Pkg().Path()] {
		return nil, false
	}
	return []node{{field: field}}, true
}

// load follows a load: of a field to the field, and of a local to every store
// into it.
func (f *flow) load(addr ssa.Value, t *trail) ([]node, bool) {
	switch addr := addr.(type) {
	case *ssa.FieldAddr:
		return f.fieldNode(fieldOf(addr.X.Type(), addr.Field))
	case *ssa.Alloc:
		return f.local(addr, t)
	case *ssa.FreeVar:
		var deps []node
		for _, closure := range f.a.idx.closures[addr.Parent()] {
			alloc, ok := closure.Bindings[indexOf(addr.Parent().FreeVars, addr)].(*ssa.Alloc)
			if !ok {
				return nil, false
			}
			more, ok := f.local(alloc, t)
			if !ok {
				return nil, false
			}
			deps = append(deps, more...)
		}
		return deps, true
	default:
		return nil, false
	}
}

func (f *flow) local(alloc *ssa.Alloc, t *trail) ([]node, bool) {
	if escapes(alloc) {
		return nil, false
	}
	var values []ssa.Value
	for _, ref := range *alloc.Referrers() {
		if store, ok := ref.(*ssa.Store); ok && store.Addr == alloc {
			values = append(values, store.Val)
		}
	}
	return f.values(t, append(values, capturedStores(alloc)...)...)
}

// sliceLiteral follows a []string literal to its elements.
func (f *flow) sliceLiteral(slice *ssa.Slice, t *trail) ([]node, bool) {
	array, ok := slice.X.(*ssa.Alloc)
	if !ok || array.Comment != "slicelit" {
		return nil, false
	}
	var elements []ssa.Value
	for _, ref := range *array.Referrers() {
		switch ref := ref.(type) {
		case *ssa.Slice, *ssa.DebugRef:
		case *ssa.IndexAddr:
			for _, use := range *ref.Referrers() {
				store, ok := use.(*ssa.Store)
				if !ok || store.Addr != ref {
					return nil, false
				}
				elements = append(elements, store.Val)
			}
		default:
			return nil, false
		}
	}
	return f.values(t, elements...)
}

// result follows a static call's result: to the origin, when it counts
// the call as a source, and otherwise, for an unexported module function,
// to what its returns are.
func (f *flow) result(call *ssa.CallCommon, index int, t *trail) ([]node, bool) {
	callee := call.StaticCallee()
	if callee == nil || call.IsInvoke() {
		return nil, false
	}
	if rendered, ok := f.origin.result(callee); ok {
		if rendered != nil {
			t.renders = append(t.renders, rendered)
		}
		return nil, true
	}
	if !f.followedResults(callee) {
		return nil, false
	}
	return []node{{fn: callee, index: index}}, true
}

// apiEnums returns the exported enums of API packages, whose String
// is a member's name.
func (a *analysis) apiEnums() map[*types.Named]bool {
	enums := map[*types.Named]bool{}
	for _, pkg := range a.module.Packages {
		if !a.api[pkg.PkgPath] {
			continue
		}
		for _, e := range enum.Declared(enum.Package{Fset: pkg.Fset, Files: pkg.Syntax, Types: pkg.Types, Info: pkg.TypesInfo}) {
			if e.Type.Obj().Exported() && (e.Ordinal() || e.Textual()) {
				enums[e.Type] = true
			}
		}
	}
	return enums
}
