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
// That argument reaches exactly as far as reflection does, so an unexported
// method is weighed against the seeds instead — the conversion operands
// and the reflect.TypeFor arguments themselves, not what the closure derived
// from them. Reflection obtains no unexported method, so a type reached only
// by derivation carries no evidence that the program can invoke one on it,
// and the exported-method argument above never applied to that case. The two
// gates are the same question asked of what each kind of method is reachable
// through.
//
// The same reach bounds the derivation itself, and RTA departs from it in two
// places. RTA also derives through the signatures of unexported methods, which
// reflection cannot call, so a type only such a signature mentions never
// materializes: an exported method on it that nothing selects or binds
// is reported reflection-live, the verdict for exactly a method RTA keeps alive
// through reflection alone. And at the pinned x/tools version RTA derives
// nothing from a type it first reaches through an alias, where reflection
// reaches everything the unaliased spelling would, so the closure derives
// from it regardless. Neither derives through a generic method's signature,
// since reflection cannot call a generic method.
//
// The closure additionally seeds from the resolved type arguments
// of reflect.TypeFor, which conjures a type descriptor no MakeInterface ever
// carried — reflect.Zero on it produces the value the operand sweep never saw.
// reflect.TypeOf needs no rule of its own: its argument is converted to any
// at the call, which is a MakeInterface the sweep already sees.
//
// It seeds, too, from what the declared surface's sealed interfaces hold.
// A consumer holding one holds an implementation behind it — the premise
// the sealed credit rests on — and the conversion that put it there is in code
// this analysis does not load, where no operand sweep reaches. Without the seed
// an implementation the seal credits would still read as never materialized,
// and an assertion from the sealed interface to another one, the within-package
// capability a seal makes possible, would find nothing behind it.
type evidence struct {
	// byOperand records the distinct conversions to an interface the built program
	// performs, keyed by operand.
	byOperand typeutil.Map
	// granted records every type the closure visited; the value says whether some
	// non-skip position reached it, which is what grants evidence.
	granted typeutil.Map
	// seeds records the types the closure started from — conversion operands,
	// resolved reflect.TypeFor arguments, and what the sealed surface holds —
	// as opposed to the types it derived from them. An unexported method
	// is credited against these alone.
	seeds typeutil.Map
	msets *typeutil.MethodSetCache
}

func newEvidence(prog *ssa.Program, inst *instantiations, sealed *sealedSurface, v view) *evidence {
	ev := &evidence{msets: &prog.MethodSets}
	for fn := range ssautil.AllFunctions(prog) {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				conversion, ok := instr.(*ssa.MakeInterface)
				if !ok || !v.admitsConversion(prog.Fset, fn, instr, conversion.X.Type(), conversion.Type()) {
					continue
				}
				ev.addConversion(conversion.X.Type(), conversion.Type())
				ev.seed(conversion.X.Type())
			}
		}
	}
	for _, typ := range reflectTypeForArguments(prog, inst) {
		ev.seed(typ)
	}
	for typ := range sealed.held {
		ev.seed(typ)
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
	granted := e.evident
	if !method.Exported() {
		// What RTA keeps alive off a derived type is its exported method set, which
		// is the whole of what reflection can reach there. Deriving a receiver into
		// the closure therefore says nothing about whether an unexported method
		// on it can be invoked, so one is credited against the seeds alone.
		granted = e.seeded
	}
	if granted(typ) || granted(types.NewPointer(typ)) {
		return true
	}
	named, ok := typ.(*types.Named)
	return ok && named.TypeArgs().Len() > 0 && granted(named.Origin())
}

// seeded reports whether the type is one the closure started from rather than
// one it derived from something else.
func (e *evidence) seeded(t types.Type) bool {
	granted, ok := e.seeds.At(t).(bool)
	return ok && granted
}

// seed records t as a starting point of the closure and derives from it.
func (e *evidence) seed(t types.Type) {
	e.seeds.Set(t, true)
	e.derive(t, false)
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

// derive adds t to the closure with the derivation rules of reflection's reach,
// which the evidence type weighs against RTA's, and grants evidence wherever
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

	for method := range e.msets.MethodSet(t).Methods() {
		fn, ok := method.Obj().(*types.Func)
		if !ok || !fn.Exported() || fn.Signature().TypeParams() != nil {
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
