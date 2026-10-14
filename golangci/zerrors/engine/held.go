package engine

import (
	"go/ast"
	"go/constant"
	"go/types"
	"maps"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// held is what a constant-derived node can hold, as far as literal-field's
// message needs to know: nothing yet, one value, or more than one.
type held struct {
	// one is the single value, as Go source writes it.
	one string

	// raw is the single value's own text, when it is a string, which is what
	// a concatenation joins.
	raw string

	known bool
	many  bool
}

func single(raw string) held {
	return held{one: strconv.Quote(raw), raw: raw, known: true}
}

func (h held) union(other held) held {
	switch {
	case !h.known && !h.many:
		return other
	case !other.known && !other.many:
		return h
	case h.many || other.many || h.one != other.one:
		return held{many: true}
	default:
		return h
	}
}

// holding works out what each constant-derived node holds. It runs after
// the fixed point, so every value it reads is one the fixed point followed.
type holding struct {
	l     *literals
	nodes map[node]held

	// zeroed holds the fields some module code makes a value of their struct
	// without writing.
	zeroed map[*types.Var]bool
}

func (l *literals) holding() *holding {
	return &holding{l: l, nodes: map[node]held{}, zeroed: l.a.zeroedFields()}
}

// field returns what a field holds: what its stores write, and its zero value
// too when some construction leaves it unwritten.
func (h *holding) field(field *types.Var) held {
	found := h.node(node{field: field})
	if h.zeroed[field] {
		zero := single("")
		if _, ok := field.Type().(*types.Slice); ok {
			zero = held{one: "nil", known: true}
		}
		found = found.union(zero)
	}
	return found
}

func (h *holding) node(n node) held {
	if found, ok := h.nodes[n]; ok {
		return found
	}
	// A node read while it is being read adds nothing to itself.
	h.nodes[n] = held{}

	var values []ssa.Value
	var found held
	switch {
	case n.field != nil:
		if len(h.l.a.idx.elementStores[n.field]) > 0 {
			found = held{many: true}
		}
		for _, store := range h.l.a.idx.fieldStores[n.field] {
			values = append(values, store.value)
		}
	case n.param != nil:
		fn := n.param.Parent()
		i := indexOf(fn.Params, n.param)
		for _, site := range h.l.a.idx.calls[fn] {
			values = append(values, site.call.Args[i])
		}
	default:
		for _, block := range n.fn.Blocks {
			if ret, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return); ok {
				values = append(values, ret.Results[n.index])
			}
		}
	}
	for _, value := range values {
		found = found.union(h.value(value, map[ssa.Value]bool{}))
	}
	h.nodes[n] = found
	return found
}

func (h *holding) value(v ssa.Value, visiting map[ssa.Value]bool) held {
	if visiting[v] {
		return held{}
	}
	visiting[v] = true
	switch v := v.(type) {
	case *ssa.Const:
		if v.IsNil() || v.Value == nil {
			return held{one: "nil", known: true}
		}
		return single(constant.StringVal(v.Value))
	case *ssa.BinOp:
		x, y := h.value(v.X, visiting), h.value(v.Y, visiting)
		if x.many || y.many || !x.known || !y.known {
			return held{many: x.many || y.many}
		}
		return single(x.raw + y.raw)
	case *ssa.Phi:
		return h.values(visiting, v.Edges...)
	case *ssa.UnOp:
		return h.load(v.X, visiting)
	case *ssa.Field:
		return h.node(node{field: fieldOf(v.X.Type(), v.Field)})
	case *ssa.Slice:
		return h.sliceLiteral(v, visiting)
	case *ssa.Parameter:
		return h.node(node{param: v})
	case *ssa.Call:
		return h.result(&v.Call, 0)
	case *ssa.Extract:
		if call, ok := v.Tuple.(*ssa.Call); ok {
			return h.result(&call.Call, v.Index)
		}
	}
	return held{many: true}
}

func (h *holding) values(visiting map[ssa.Value]bool, values ...ssa.Value) held {
	var found held
	for _, value := range values {
		found = found.union(h.value(value, visiting))
	}
	return found
}

func (h *holding) load(addr ssa.Value, visiting map[ssa.Value]bool) held {
	switch addr := addr.(type) {
	case *ssa.FieldAddr:
		return h.node(node{field: fieldOf(addr.X.Type(), addr.Field)})
	case *ssa.Alloc:
		return h.local(addr, visiting)
	case *ssa.FreeVar:
		var found held
		for _, closure := range h.l.a.idx.closures[addr.Parent()] {
			if alloc, ok := closure.Bindings[indexOf(addr.Parent().FreeVars, addr)].(*ssa.Alloc); ok {
				found = found.union(h.local(alloc, visiting))
			}
		}
		return found
	default:
		return held{many: true}
	}
}

func (h *holding) local(alloc *ssa.Alloc, visiting map[ssa.Value]bool) held {
	var values []ssa.Value
	for _, ref := range *alloc.Referrers() {
		if store, ok := ref.(*ssa.Store); ok && store.Addr == alloc {
			values = append(values, store.Val)
		}
	}
	return h.values(visiting, append(values, capturedStores(alloc)...)...)
}

// sliceLiteral reads a []string literal whose every element holds one value
// as that one list.
func (h *holding) sliceLiteral(slice *ssa.Slice, visiting map[ssa.Value]bool) held {
	array, ok := slice.X.(*ssa.Alloc)
	if !ok {
		return held{many: true}
	}
	elements := map[int64]string{}
	for _, ref := range *array.Referrers() {
		addr, ok := ref.(*ssa.IndexAddr)
		if !ok {
			continue
		}
		index, ok := addr.Index.(*ssa.Const)
		if !ok {
			return held{many: true}
		}
		for _, use := range *addr.Referrers() {
			store, ok := use.(*ssa.Store)
			if !ok {
				continue
			}
			element := h.value(store.Val, visiting)
			if !element.known || element.many {
				return held{many: element.many}
			}
			elements[index.Int64()] = element.one
		}
	}
	rendered := make([]string, 0, len(elements))
	for _, key := range slices.Sorted(maps.Keys(elements)) {
		rendered = append(rendered, elements[key])
	}
	return held{one: "[]string{" + strings.Join(rendered, ", ") + "}", known: true}
}

// result reads a static call's result: an enum member's name can be any
// member's, and an unexported module function's result is what its returns
// hold.
func (h *holding) result(call *ssa.CallCommon, index int) held {
	callee := call.StaticCallee()
	if callee == nil || h.l.memberName(callee) {
		return held{many: true}
	}
	return h.node(node{fn: callee, index: index})
}

// zeroedFields returns the fields some module code makes a value of their
// struct without writing: a composite literal leaving them out, new,
// or a variable declared with the struct's type and no value.
func (a *analysis) zeroedFields() map[*types.Var]bool {
	zeroed := map[*types.Var]bool{}
	all := func(t types.Type) {
		if st, ok := types.Unalias(t).Underlying().(*types.Struct); ok {
			for field := range st.Fields() {
				zeroed[field] = true
			}
		}
	}
	for _, pkg := range a.module.Packages {
		info := pkg.TypesInfo
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.CompositeLit:
					unwritten(info.TypeOf(n), n, zeroed)
				case *ast.CallExpr:
					if id, ok := ast.Unparen(n.Fun).(*ast.Ident); ok && len(n.Args) == 1 {
						if builtin, ok := info.Uses[id].(*types.Builtin); ok && builtin.Name() == "new" {
							all(info.TypeOf(n.Args[0]))
						}
					}
				case *ast.ValueSpec:
					if n.Type != nil && len(n.Values) == 0 {
						all(info.TypeOf(n.Type))
					}
				}
				return true
			})
		}
	}
	return zeroed
}

// unwritten records the fields of a struct composite literal that it leaves
// out. A positional literal writes every field.
func unwritten(t types.Type, lit *ast.CompositeLit, zeroed map[*types.Var]bool) {
	if t == nil {
		return
	}
	st, ok := types.Unalias(t).Underlying().(*types.Struct)
	if !ok {
		return
	}
	written := map[string]bool{}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return
		}
		if key, ok := kv.Key.(*ast.Ident); ok {
			written[key.Name] = true
		}
	}
	for field := range st.Fields() {
		if !written[field.Name()] {
			zeroed[field] = true
		}
	}
}
