package wholemodule

import "golang.org/x/tools/go/ssa"

// VariadicArgs returns the operands a call passes in its variadic parameter,
// in order, as the call wrote them: an operand converted to an interface type
// is returned as that conversion. It reports false for a call to a function
// that is not variadic, and for one that passes an existing slice with ...,
// whose operands the call does not show.
//
// SSA packs variadic operands into an array allocated for the call and passes
// a slice of it, so the operands are the values stored into that array's
// elements.
func VariadicArgs(call *ssa.CallCommon) ([]ssa.Value, bool) {
	sig := call.Signature()
	if sig == nil || !sig.Variadic() || len(call.Args) == 0 {
		return nil, false
	}
	last := call.Args[len(call.Args)-1]
	if c, ok := last.(*ssa.Const); ok && c.IsNil() {
		return nil, true
	}
	slice, ok := last.(*ssa.Slice)
	if !ok {
		return nil, false
	}
	alloc, ok := slice.X.(*ssa.Alloc)
	if !ok || alloc.Comment != "varargs" {
		return nil, false
	}

	var args []ssa.Value
	for _, ref := range *alloc.Referrers() {
		addr, ok := ref.(*ssa.IndexAddr)
		if !ok {
			continue
		}
		index, ok := addr.Index.(*ssa.Const)
		if !ok {
			return nil, false
		}
		i := int(index.Int64())
		for _, use := range *addr.Referrers() {
			store, ok := use.(*ssa.Store)
			if !ok || store.Addr != addr {
				continue
			}
			for len(args) <= i {
				args = append(args, nil)
			}
			args[i] = store.Val
		}
	}
	return args, true
}
