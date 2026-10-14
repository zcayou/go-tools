package engine

import (
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/zcayou/go-tools/golangci/internal/enum"
)

// node is one place constant-derivation is decided for: a struct field,
// a parameter, or a function's result. Exactly one of field, param and fn
// is set.
type node struct {
	field *types.Var
	param *ssa.Parameter
	fn    *ssa.Function
	index int
}

// literals decides which nodes only ever hold constants. It is the greatest
// fixed point: every node starts out constant-derived, and a node fed anything
// that is not loses it, as does every node depending on one that lost it,
// until nothing changes.
type literals struct {
	a *analysis

	// deps are the nodes each node's values are constant-derived through.
	deps map[node][]node

	// dynamic holds the nodes fed some value that is not constant-derived
	// whatever the other nodes hold.
	dynamic map[node]bool

	// enums holds the exported enums of API packages, whose String() is a member's
	// name.
	enums map[*types.Named]bool
}

// literalFields reports each string or []string field of a module error type
// that has at least one store and only ever holds constants.
func (a *analysis) literalFields() []Finding {
	l := &literals{
		a:       a,
		deps:    map[node][]node{},
		dynamic: map[node]bool{},
		enums:   map[*types.Named]bool{},
	}
	for _, pkg := range a.module.Packages {
		if !a.api[pkg.PkgPath] {
			continue
		}
		for _, e := range enum.Declared(enum.Package{Fset: pkg.Fset, Files: pkg.Syntax, Types: pkg.Types, Info: pkg.TypesInfo}) {
			if e.Type.Obj().Exported() && (e.Ordinal() || e.Textual()) {
				l.enums[e.Type] = true
			}
		}
	}

	var findings []Finding
	var candidates []*types.Var
	owners := map[*types.Var]*types.Named{}
	for _, named := range a.idx.types {
		if errorNamed(named) == nil {
			continue
		}
		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for field := range st.Fields() {
			if textual(field.Type()) && l.stored(field) {
				candidates = append(candidates, field)
				owners[field] = named
				l.build(node{field: field})
			}
		}
	}
	lost := l.settle()
	holding := l.holding()
	for _, field := range candidates {
		if lost[node{field: field}] {
			continue
		}
		owner := typeName(owners[field]) + "." + field.Name()
		message := owner + " only ever holds constants; give it an enum type"
		// One value is no choice an enum could name: the field says nothing.
		if held := holding.field(field); held.known && !held.many {
			message = owner + " only ever holds " + held.one + ", so it carries no information; remove it"
		}
		findings = append(findings, Finding{
			Pos:     a.module.Fset.PositionFor(field.Pos(), false),
			Rule:    RuleLiteralField,
			Subject: owner,
			Message: message,
		})
	}
	return findings
}

// textual reports whether a field's type is exactly string or []string.
// A named string type is already a typing decision, which zenum governs.
func textual(t types.Type) bool {
	if slice, ok := t.(*types.Slice); ok {
		t = slice.Elem()
	}
	return types.Identical(t, types.Typ[types.String])
}

// stored reports whether some module code stores to a field.
func (l *literals) stored(field *types.Var) bool {
	return len(l.a.idx.fieldStores[field]) > 0
}

// build records the dependencies of a node and of every node it reaches.
func (l *literals) build(start node) {
	pending := []node{start}
	for len(pending) > 0 {
		n := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if _, built := l.deps[n]; built || l.dynamic[n] {
			continue
		}
		deps, ok := l.sources(n)
		l.deps[n] = deps
		if !ok {
			l.dynamic[n] = true
			continue
		}
		pending = append(pending, deps...)
	}
}

// settle returns the nodes that lose constant-derivation.
func (l *literals) settle() map[node]bool {
	dependents := map[node][]node{}
	for n, deps := range l.deps {
		for _, dep := range deps {
			dependents[dep] = append(dependents[dep], n)
		}
	}
	lost := map[node]bool{}
	var pending []node
	for n := range l.dynamic {
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

// sources returns the nodes a node's values depend on, and false when one
// of them is not constant-derived whatever the nodes hold.
func (l *literals) sources(n node) ([]node, bool) {
	var values []ssa.Value
	switch {
	case n.field != nil:
		if l.a.idx.unseen[n.field] {
			return nil, false
		}
		for _, store := range slices.Concat(l.a.idx.fieldStores[n.field], l.a.idx.elementStores[n.field]) {
			values = append(values, store.value)
		}
	case n.param != nil:
		fn := n.param.Parent()
		if !l.followedParams(fn) {
			return nil, false
		}
		i := indexOf(fn.Params, n.param)
		for _, site := range l.a.idx.calls[fn] {
			values = append(values, site.call.Args[i])
		}
	default:
		for _, block := range n.fn.Blocks {
			if ret, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return); ok {
				values = append(values, ret.Results[n.index])
			}
		}
	}

	var deps []node
	for _, value := range values {
		more, ok := l.value(value, map[ssa.Value]bool{})
		if !ok {
			return nil, false
		}
		deps = append(deps, more...)
	}
	return deps, true
}

// followedParams reports whether a function's parameters are followed to its
// callers: an unexported module function, called only statically, that some
// module code calls. A generic body's parameters are not followed: what
// they hold depends on its type arguments.
func (l *literals) followedParams(fn *ssa.Function) bool {
	obj := declaredFunc(fn)
	if obj == nil || obj.Exported() || synthesized(fn) || parameterized(fn) || !l.a.module.Declares(obj) {
		return false
	}
	return !l.a.idx.escapes[fn] && len(l.a.idx.dispatch[obj]) == 0 && len(l.a.idx.calls[fn]) > 0
}

// followedResults reports whether a function's results are followed:
// an unexported module function with a body.
func (l *literals) followedResults(fn *ssa.Function) bool {
	obj := declaredFunc(fn)
	return obj != nil && !obj.Exported() && l.a.module.Declares(obj) && len(fn.Blocks) > 0 && !parameterized(fn) &&
		!synthesized(fn)
}

// value returns the nodes a value is constant-derived through, and false when
// it is not constant-derived whatever they hold.
func (l *literals) value(v ssa.Value, visiting map[ssa.Value]bool) ([]node, bool) {
	if visiting[v] {
		return nil, true
	}
	visiting[v] = true
	switch v := v.(type) {
	case *ssa.Const:
		return nil, true
	case *ssa.BinOp:
		if v.Op != token.ADD {
			return nil, false
		}
		return l.values(visiting, v.X, v.Y)
	case *ssa.Phi:
		return l.values(visiting, v.Edges...)
	case *ssa.UnOp:
		if v.Op != token.MUL {
			return nil, false
		}
		return l.load(v.X, visiting)
	case *ssa.Field:
		return l.fieldNode(fieldOf(v.X.Type(), v.Field))
	case *ssa.Slice:
		return l.sliceLiteral(v, visiting)
	case *ssa.Parameter:
		if !l.followedParams(v.Parent()) {
			return nil, false
		}
		return []node{{param: v}}, true
	case *ssa.Call:
		return l.result(&v.Call, 0)
	case *ssa.Extract:
		if call, ok := v.Tuple.(*ssa.Call); ok {
			return l.result(&call.Call, v.Index)
		}
	}
	return nil, false
}

func (l *literals) values(visiting map[ssa.Value]bool, values ...ssa.Value) ([]node, bool) {
	var deps []node
	for _, value := range values {
		more, ok := l.value(value, visiting)
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
func (l *literals) fieldNode(field *types.Var) ([]node, bool) {
	if !l.a.module.Declares(field) || !textual(field.Type()) || field.Exported() && l.a.api[field.Pkg().Path()] {
		return nil, false
	}
	return []node{{field: field}}, true
}

// load follows a load: of a field to the field, and of a local to every store
// into it.
func (l *literals) load(addr ssa.Value, visiting map[ssa.Value]bool) ([]node, bool) {
	switch addr := addr.(type) {
	case *ssa.FieldAddr:
		return l.fieldNode(fieldOf(addr.X.Type(), addr.Field))
	case *ssa.Alloc:
		return l.local(addr, visiting)
	case *ssa.FreeVar:
		var deps []node
		for _, closure := range l.a.idx.closures[addr.Parent()] {
			alloc, ok := closure.Bindings[indexOf(addr.Parent().FreeVars, addr)].(*ssa.Alloc)
			if !ok {
				return nil, false
			}
			more, ok := l.local(alloc, visiting)
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

func (l *literals) local(alloc *ssa.Alloc, visiting map[ssa.Value]bool) ([]node, bool) {
	if escapes(alloc) {
		return nil, false
	}
	var values []ssa.Value
	for _, ref := range *alloc.Referrers() {
		if store, ok := ref.(*ssa.Store); ok && store.Addr == alloc {
			values = append(values, store.Val)
		}
	}
	return l.values(visiting, append(values, capturedStores(alloc)...)...)
}

// sliceLiteral follows a []string literal to its elements.
func (l *literals) sliceLiteral(slice *ssa.Slice, visiting map[ssa.Value]bool) ([]node, bool) {
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
	return l.values(visiting, elements...)
}

// result follows a static call's result: the name of an exported API enum's
// member is constant, and an unexported module function's result is what its
// returns are.
func (l *literals) result(call *ssa.CallCommon, index int) ([]node, bool) {
	callee := call.StaticCallee()
	if callee == nil || call.IsInvoke() {
		return nil, false
	}
	if l.memberName(callee) {
		return nil, true
	}
	if !l.followedResults(callee) {
		return nil, false
	}
	return []node{{fn: callee, index: index}}, true
}

// memberName reports whether fn is the String method naming an exported API
// enum's members. An internal type's String is a display string, and is not
// followed.
func (l *literals) memberName(fn *ssa.Function) bool {
	obj, ok := fn.Object().(*types.Func)
	if !ok || obj.Signature().Recv() == nil {
		return false
	}
	named, ok := types.Unalias(pointee(obj.Signature().Recv().Type())).(*types.Named)
	return ok && l.enums[named] && enum.Name(named) == obj
}
