package engine

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"golang.org/x/tools/go/types/typeutil"
)

// evidence is the one answer to "can this program hold a value of that type
// behind an interface", shared by every credit gate. Which concrete types
// the program materializes is what separates a bind from structural
// satisfaction: a dependency's interface that some type happens to fit is not
// evidence that anything ever held that type behind it, and every program
// imports enough of the standard library for structural satisfaction alone
// to credit almost anything.
//
// The evidence is a derived-type closure, not the conversion operands alone,
// because liveness is decided against the closure: RTA derives struct fields,
// pointer, slice, array, chan and map constituents, and the parameter
// and result types of signatures from every MakeInterface operand, and treats
// every derived type as a runtime type reflection can produce. A credit gate
// that consulted only the operands would then refuse exactly the methods
// liveness keeps alive through derivation — alive but uncreditable
// is the false-positive verdict space. Under the linter's contract the closure
// is not an approximation being loosened: a String method on a type
// in the closure is invoked by an fmt call on the boxed value, so crediting
// it is correct.
//
// The closure additionally seeds from the resolved type arguments
// of reflect.TypeFor, which conjures a type descriptor no MakeInterface ever
// carried — reflect.Zero on it produces the value the operand sweep never saw.
// reflect.TypeOf needs no rule of its own: its argument is converted to any
// at the call, which is a MakeInterface the sweep already sees.
type evidence struct {
	// byOperand records the distinct conversions to an interface the built program
	// performs, keyed by operand.
	byOperand typeutil.Map
	// granted records every type the closure visited; the value says whether some
	// non-skip position reached it, which is what grants evidence.
	granted typeutil.Map
	msets   *typeutil.MethodSetCache
}

func newEvidence(prog *ssa.Program, inst *instantiations) *evidence {
	ev := &evidence{msets: &prog.MethodSets}
	for fn := range ssautil.AllFunctions(prog) {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if conversion, ok := instr.(*ssa.MakeInterface); ok {
					ev.addConversion(conversion.X.Type(), conversion.Type())
					ev.derive(conversion.X.Type(), false)
				}
			}
		}
	}
	for _, typ := range reflectTypeForArguments(prog, inst) {
		ev.derive(typ, false)
	}
	return ev
}

// conversions yields each distinct MakeInterface (operand, target) pair once.
// Interface-to-interface conversions are not here: they move a value between
// interfaces rather than materialize one behind an interface, which makes them
// flow edges rather than evidence.
func (e *evidence) conversions(yield func(operand, iface types.Type) bool) {
	done := false
	e.byOperand.Iterate(func(operand types.Type, value any) {
		if done {
			return
		}
		value.(*typeutil.Map).Iterate(func(iface types.Type, _ any) {
			if !done && !yield(operand, iface) {
				done = true
			}
		})
	})
}

// materialized reports whether the program can hold a value of the method's
// receiver type behind an interface: the receiver, its pointer or element form,
// or — for a method of a generic type — the shared generic origin appears
// in the closure. A declared method's receiver names the generic instantiated
// with its own type parameters, a form no program value ever has, while
// the closure holds concrete instantiations; the recorded origin is where
// the two meet.
func (e *evidence) materialized(method *types.Func) bool {
	recv := method.Signature().Recv()
	if recv == nil {
		return false
	}
	typ := types.Unalias(recv.Type())
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = types.Unalias(pointer.Elem())
	}
	if e.evident(typ) || e.evident(types.NewPointer(typ)) {
		return true
	}
	named, ok := typ.(*types.Named)
	return ok && named.TypeArgs().Len() > 0 && e.evident(named.Origin())
}

func (e *evidence) evident(t types.Type) bool {
	granted, ok := e.granted.At(t).(bool)
	return ok && granted
}

func (e *evidence) addConversion(operand, iface types.Type) {
	ifaces, _ := e.byOperand.At(operand).(*typeutil.Map)
	if ifaces == nil {
		ifaces = new(typeutil.Map)
		e.byOperand.Set(operand, ifaces)
	}
	ifaces.Set(iface, true)
}

// derive adds t to the closure with the derivation rules of RTA's
// addRuntimeType at the pinned x/tools version, and grants evidence wherever
// a non-skip position reaches a type. A skip position — a named type's
// underlying, a signature's parameter or result tuple — is traversed but
// granted nothing: reflection cannot obtain the method set of what sits there.
// The memo upgrades when a later visit reaches a type in non-skip position, so
// the grant never depends on which visit came first. Type parameters are left
// alone entirely: a generic body's operands carry their evidence
// in the instantiated form.
func (e *evidence) derive(t types.Type, skip bool) {
	t = types.Unalias(t)
	if _, ok := t.(*types.TypeParam); ok {
		return
	}
	if prev, ok := e.granted.At(t).(bool); ok {
		if !skip && !prev {
			e.record(t, true)
		}
		return
	}
	e.record(t, !skip)

	if named := namedForm(t); named != nil && named.Obj().Pkg() == nil {
		// The built-in error type: nothing to derive, matching RTA.
		return
	}

	for method := range e.msets.MethodSet(t).Methods() {
		fn, ok := method.Obj().(*types.Func)
		if !ok || !fn.Exported() {
			continue
		}
		e.derive(fn.Signature().Params(), true)
		e.derive(fn.Signature().Results(), true)
	}

	e.deriveShape(t)
}

// record notes the visit, and on a grant records a named instantiation's origin
// alongside it, the form materialized resolves generic receivers to.
func (e *evidence) record(t types.Type, granted bool) {
	e.granted.Set(t, granted)
	if !granted {
		return
	}
	if named, ok := t.(*types.Named); ok && named.TypeArgs().Len() > 0 {
		e.granted.Set(named.Origin(), true)
	}
}

func (e *evidence) deriveShape(t types.Type) {
	switch t := t.(type) {
	case *types.Pointer:
		e.derive(t.Elem(), false)
	case *types.Slice:
		e.derive(t.Elem(), false)
	case *types.Chan:
		e.derive(t.Elem(), false)
	case *types.Array:
		e.derive(t.Elem(), false)
	case *types.Map:
		e.derive(t.Key(), false)
		e.derive(t.Elem(), false)
	case *types.Signature:
		e.derive(t.Params(), true)
		e.derive(t.Results(), true)
	case *types.Named:
		// A pointer to a named type can be derived from the named type via
		// reflection, and carries its own method set. The underlying is skip:
		// reflection reaches an embedded field's type, never the underlying as such.
		e.derive(types.NewPointer(t), false)
		e.derive(t.Underlying(), true)
	case *types.Struct:
		for field := range t.Fields() {
			e.derive(field.Type(), false)
		}
	case *types.Tuple:
		for field := range t.Variables() {
			e.derive(field.Type(), false)
		}
	}
	// Basic types and interfaces derive nothing beyond the method-set recursion
	// above.
}

// namedForm unwraps to the named type behind t, through one pointer, the way
// RTA's built-in-error check does.
func namedForm(t types.Type) *types.Named {
	switch t := t.(type) {
	case *types.Named:
		return t
	case *types.Pointer:
		named, _ := types.Unalias(t.Elem()).(*types.Named)
		return named
	}
	return nil
}

// reflectTypeForArguments returns the first type argument of every resolved
// concrete instantiation of reflect.TypeFor, dependencies' instantiations
// included and parametric ones resolved transitively — the kmbldr corpus
// resolves TypeFor three generics deep before a concrete vector appears.
func reflectTypeForArguments(prog *ssa.Program, inst *instantiations) []types.Type {
	reflectPkg := prog.ImportedPackage("reflect")
	if reflectPkg == nil {
		return nil
	}
	fn, ok := reflectPkg.Members["TypeFor"].(*ssa.Function)
	if !ok || fn.Object() == nil {
		return nil
	}
	var args []types.Type
	for _, vector := range inst.vectors(fn.Object()) {
		if len(vector) > 0 {
			args = append(args, vector[0])
		}
	}
	return args
}
