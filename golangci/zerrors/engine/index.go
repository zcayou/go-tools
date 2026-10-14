package engine

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
)

// index is what every rule reads of the module's code, gathered in one walk
// over every function the module's source builds: declared functions
// and methods, their closures, the instances of the module's generics,
// and the wrappers the builder makes for them.
type index struct {
	// funcs are those functions, each with a body, in a stable order.
	funcs []*ssa.Function

	// declared maps each declared module function to its SSA function, generic
	// ones as their origin.
	declared map[*types.Func]*ssa.Function

	// fieldStores are the values stored into each struct field, by the field's
	// declaration.
	fieldStores map[*types.Var][]site

	// elementStores are the values stored into the elements of a slice loaded
	// from each field.
	elementStores map[*types.Var][]site

	// unseen holds the fields module code may write in ways no store shows:
	// through the field's address put to another use, or through a conversion
	// from another struct type; and the exported fields foreign code may write,
	// handed the struct.
	unseen map[*types.Var]bool

	// globalStores are the values stored into each package-level variable.
	globalStores map[*ssa.Global][]site

	// conversions maps each interface type module code converts a concrete type
	// to onto the concrete types converted.
	conversions typeutil.Map

	// calls are the static calls of each function.
	calls map[*ssa.Function][]callSite

	// invokes are the calls through each interface method.
	invokes map[*types.Func][]callSite

	// dispatch maps each module method an interface call can reach, as its
	// origin, onto the interface methods it answers.
	dispatch map[*types.Func][]answer

	// escapes holds the functions put to some use other than a static call,
	// whose callers are therefore not all known.
	escapes map[*ssa.Function]bool

	// closures are the closures each anonymous function is made into.
	closures map[*ssa.Function][]*ssa.MakeClosure

	// types are the module's defined, non-interface types, generic ones included.
	types []*types.Named
}

// site is a value as it appears in the function holding it. A constant
// belongs to no function, so the value alone cannot say.
type site struct {
	fn    *ssa.Function
	value ssa.Value
}

// callSite is a call as it appears in the function making it.
type callSite struct {
	caller *ssa.Function
	call   *ssa.CallCommon
}

// answer is an interface method a module method is reached through,
// and the interface type it is reached through.
type answer struct {
	iface  types.Type
	method *types.Func
}

func newIndex(a *analysis) *index {
	idx := &index{
		declared:      map[*types.Func]*ssa.Function{},
		fieldStores:   map[*types.Var][]site{},
		elementStores: map[*types.Var][]site{},
		unseen:        map[*types.Var]bool{},
		globalStores:  map[*ssa.Global][]site{},
		calls:         map[*ssa.Function][]callSite{},
		invokes:       map[*types.Func][]callSite{},
		dispatch:      map[*types.Func][]answer{},
		escapes:       map[*ssa.Function]bool{},
		closures:      map[*ssa.Function][]*ssa.MakeClosure{},
	}

	idx.gather(a)
	slices.SortFunc(idx.funcs, func(x, y *ssa.Function) int {
		return cmp.Or(cmp.Compare(x.Pos(), y.Pos()), strings.Compare(x.String(), y.String()))
	})
	slices.SortFunc(idx.types, func(x, y *types.Named) int { return cmp.Compare(x.Obj().Pos(), y.Obj().Pos()) })

	for _, fn := range idx.funcs {
		idx.scan(a, fn)
	}
	idx.blankConversions(a)
	idx.eachConversion(func(iface types.Type, concretes []types.Type) {
		for _, concrete := range concretes {
			for method := range interfaceMethods(iface) {
				if fn := methodOf(concrete, method); fn != nil {
					idx.dispatch[fn.Origin()] = append(idx.dispatch[fn.Origin()], answer{iface: iface, method: method})
				}
			}
		}
	})
	return idx
}

// gather collects the module's functions: its declared functions and methods,
// and everything their bodies reach that the module's source gave a body,
// along with its defined types.
func (idx *index) gather(a *analysis) {
	visit := (&gatherer{a: a, idx: idx, seen: map[*ssa.Function]bool{}}).visit
	for _, pkg := range a.module.Packages {
		for _, member := range a.prog.Packages[pkg.Types].Members {
			if fn, ok := member.(*ssa.Function); ok {
				visit(fn)
			}
		}
		for _, obj := range pkg.TypesInfo.Defs {
			switch obj := obj.(type) {
			case *types.Func:
				if fn := a.prog.FuncValue(obj); fn != nil {
					idx.declared[obj] = fn
					visit(fn)
				}
			case *types.TypeName:
				if named, ok := obj.Type().(*types.Named); ok && !obj.IsAlias() && !types.IsInterface(named) {
					idx.types = append(idx.types, named)
				}
			}
		}
	}
	// A method reached only through an interface is in no body's operands.
	for _, t := range a.prog.RuntimeTypes() {
		if types.IsInterface(t) {
			continue
		}
		for sel := range a.prog.MethodSets.MethodSet(t).Methods() {
			visit(methodValue(a.prog.Program, sel))
		}
	}
}

// methodValue returns the function a method selection runs, or nil for one
// that has no single body to build: a method declaring type parameters of its
// own, which only its instantiations have, or a method of a generic type
// selected before instantiation. Asking the builder for either panics.
func methodValue(prog *ssa.Program, sel *types.Selection) *ssa.Function {
	if fn, ok := sel.Obj().(*types.Func); !ok || fn.Signature().TypeParams().Len() > 0 {
		return nil
	}
	if named, ok := types.Unalias(pointee(sel.Recv())).(*types.Named); ok &&
		named.TypeParams().Len() > 0 && named.TypeArgs().Len() == 0 {
		return nil
	}
	return prog.MethodValue(sel)
}

// gatherer walks from each function to every function its body reaches.
type gatherer struct {
	a    *analysis
	idx  *index
	seen map[*ssa.Function]bool
}

func (g *gatherer) visit(fn *ssa.Function) {
	if fn == nil || g.seen[fn] || !g.a.moduleCode(fn) {
		return
	}
	g.seen[fn] = true
	g.idx.funcs = append(g.idx.funcs, fn)
	for _, anon := range fn.AnonFuncs {
		g.visit(anon)
	}
	var buf [10]*ssa.Value
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			for _, op := range instr.Operands(buf[:0]) {
				if callee, ok := (*op).(*ssa.Function); ok {
					g.visit(callee)
				}
			}
		}
	}
}

// moduleCode reports whether fn has a body the module's source gave it,
// directly or as a wrapper or instance of a module declaration.
func (a *analysis) moduleCode(fn *ssa.Function) bool {
	if len(fn.Blocks) == 0 {
		return false
	}
	pkg := home(fn)
	return pkg != nil && a.module.Package(pkg) != nil
}

// home is the package whose source fn comes from.
func home(fn *ssa.Function) *types.Package {
	top := topLevel(fn)
	if origin := top.Origin(); origin != nil {
		top = origin
	}
	if top.Pkg != nil {
		return top.Pkg.Pkg
	}
	if obj := top.Object(); obj != nil {
		return obj.Pkg()
	}
	return nil
}

// topLevel is the function a closure is nested in, or fn itself.
func topLevel(fn *ssa.Function) *ssa.Function {
	for fn.Parent() != nil {
		fn = fn.Parent()
	}
	return fn
}

// parameterized reports whether fn's body is written over type parameters:
// a generic declaration's own body, or a wrapper instantiating one with type
// parameters. Such a body never runs; its instances do.
func parameterized(fn *ssa.Function) bool {
	top := topLevel(fn)
	if strings.HasPrefix(top.Synthetic, "instantiation wrapper") {
		return true
	}
	return top.TypeParams().Len() > 0 && len(top.TypeArgs()) == 0
}

// synthesized reports whether the builder made fn up rather than building
// it from a declaration or an instance of one: a wrapper, a thunk, a bound
// method or a package initializer. Nothing calls such a function by its own
// name, so its parameters and results stand for whatever it wraps.
func synthesized(fn *ssa.Function) bool {
	return fn.Synthetic != "" && !strings.HasPrefix(fn.Synthetic, "instance of")
}

func (idx *index) scan(a *analysis, fn *ssa.Function) {
	var buf [10]*ssa.Value
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			idx.instruction(a, fn, instr)
			var callee ssa.Value
			if call, ok := instr.(ssa.CallInstruction); ok {
				callee = idx.call(a, fn, call)
			}
			for _, op := range instr.Operands(buf[:0]) {
				if used, ok := (*op).(*ssa.Function); ok && *op != callee {
					idx.escapes[used] = true
				}
			}
		}
	}
}

// instruction records what one instruction of fn shows apart from any call
// it makes: stores, conversions, closures and fields written out of sight.
func (idx *index) instruction(a *analysis, fn *ssa.Function, instr ssa.Instruction) {
	switch instr := instr.(type) {
	case *ssa.Store:
		idx.store(fn, instr)
	case *ssa.MakeInterface:
		if !parameterized(fn) {
			idx.convert(instr.X.Type(), instr.Type())
		}
		// Behind an interface with no methods, code that does not know the type may
		// write through the pointer by reflection: decoding into it,
		// as json.Unmarshal does. Behind any other, only the type's own methods
		// can, whose stores are seen.
		if iface, ok := instr.Type().Underlying().(*types.Interface); ok && iface.NumMethods() == 0 {
			idx.handOff(a, instr.X.Type())
		}
	case *ssa.MakeClosure:
		if anon, ok := instr.Fn.(*ssa.Function); ok && anon.Parent() != nil {
			idx.closures[anon] = append(idx.closures[anon], instr)
		}
	case *ssa.FieldAddr:
		if escapesField(instr) {
			idx.unseen[fieldOf(instr.X.Type(), instr.Field)] = true
		}
	case *ssa.ChangeType:
		// A struct converted from another struct type holds fields no store
		// into the target's shows.
		if !types.Identical(instr.X.Type(), instr.Type()) {
			idx.writtenUnseen(a, pointee(instr.Type()), everyField)
		}
	case *ssa.Convert:
		idx.writtenUnseen(a, pointee(instr.X.Type()), everyField)
	}
}

// call records a call fn makes, and returns its callee when the call is not
// an invoke.
func (idx *index) call(a *analysis, fn *ssa.Function, call ssa.CallInstruction) ssa.Value {
	common := call.Common()
	idx.foreignCall(a, common, call.Value())
	if common.IsInvoke() {
		idx.invokes[common.Method] = append(idx.invokes[common.Method], callSite{caller: fn, call: common})
		return nil
	}
	if target, ok := common.Value.(*ssa.Function); ok {
		idx.calls[target] = append(idx.calls[target], callSite{caller: fn, call: common})
	}
	return common.Value
}

// foreignCall records the module structs a call to foreign code may write
// unseen: those it is handed a pointer to, and those it makes and returns.
// Foreign code cannot name an unexported field, nor set one by reflection,
// so it writes only the exported ones.
func (idx *index) foreignCall(a *analysis, call *ssa.CallCommon, result *ssa.Call) {
	if call.IsInvoke() {
		if named, ok := types.Unalias(call.Value.Type()).(*types.Named); !ok || a.module.Declares(named.Obj()) {
			return
		}
	} else if callee := call.StaticCallee(); callee == nil || len(callee.Blocks) > 0 {
		return
	}
	for _, arg := range call.Args {
		idx.handOff(a, arg.Type())
	}
	if result == nil {
		return
	}
	if tuple, ok := result.Type().(*types.Tuple); ok {
		for v := range tuple.Variables() {
			idx.writtenUnseen(a, v.Type(), exportedField)
		}
		return
	}
	idx.writtenUnseen(a, result.Type(), exportedField)
}

// handOff records a value of type t given to code that does not know the type
// and may write through it by reflection: a pointer's target then has its
// exported fields written unseen.
func (idx *index) handOff(a *analysis, t types.Type) {
	if pointer, ok := types.Unalias(t).Underlying().(*types.Pointer); ok {
		idx.writtenUnseen(a, pointer.Elem(), exportedField)
	}
}

// fieldReach says of a struct field whether a writer reaches it, and whether
// it reaches the fields of a struct the field holds by value.
type fieldReach func(field *types.Var) (writes, descends bool)

// everyField is module code's reach: every field.
func everyField(*types.Var) (writes, descends bool) {
	return true, true
}

// exportedField is the reach of code that cannot name an unexported field,
// foreign code and reflection: the exported fields, and the fields an embedded
// one promotes.
func exportedField(field *types.Var) (writes, descends bool) {
	return field.Exported(), field.Exported() || field.Embedded()
}

// writtenUnseen records the fields of a module struct type a writer reaches,
// and of the structs it holds by value, as written in ways no store shows.
func (idx *index) writtenUnseen(a *analysis, t types.Type, reach fieldReach) {
	seen := map[types.Type]bool{}
	var mark func(types.Type)
	mark = func(t types.Type) {
		t = types.Unalias(t)
		if seen[t] {
			return
		}
		seen[t] = true
		switch u := t.Underlying().(type) {
		case *types.Array:
			mark(u.Elem())
		case *types.Struct:
			for field := range u.Fields() {
				writes, descends := reach(field)
				if writes && a.module.Declares(field) {
					idx.unseen[field.Origin()] = true
				}
				if descends {
					mark(field.Type())
				}
			}
		}
	}
	mark(t)
}

func (idx *index) store(fn *ssa.Function, store *ssa.Store) {
	switch addr := store.Addr.(type) {
	case *ssa.FieldAddr:
		field := fieldOf(addr.X.Type(), addr.Field)
		idx.fieldStores[field] = append(idx.fieldStores[field], site{fn, store.Val})
	case *ssa.Global:
		idx.globalStores[addr] = append(idx.globalStores[addr], site{fn, store.Val})
	case *ssa.IndexAddr:
		if field := loadedField(addr.X); field != nil {
			idx.elementStores[field] = append(idx.elementStores[field], site{fn, store.Val})
		}
	}
}

func (idx *index) convert(concrete, iface types.Type) {
	if types.IsInterface(concrete) {
		return
	}
	known, _ := idx.conversions.At(iface).([]types.Type)
	if slices.ContainsFunc(known, func(t types.Type) bool { return types.Identical(t, concrete) }) {
		return
	}
	idx.conversions.Set(iface, append(known, concrete))
}

// blankConversions records the conversions a blank variable declares, such
// as var _ io.Writer = (*T)(nil). The builder emits no code for one, since
// it has no effect at run time, but it states that T answers to the interface.
func (idx *index) blankConversions(a *analysis) {
	for _, pkg := range a.module.Packages {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				spec, ok := n.(*ast.ValueSpec)
				if !ok || spec.Type == nil || len(spec.Values) != len(spec.Names) {
					return true
				}
				iface := pkg.TypesInfo.TypeOf(spec.Type)
				if iface == nil || !types.IsInterface(iface) {
					return true
				}
				for i, name := range spec.Names {
					if name.Name != "_" {
						continue
					}
					value := pkg.TypesInfo.TypeOf(spec.Values[i])
					if basic, ok := value.(*types.Basic); value == nil || ok && basic.Kind() == types.UntypedNil {
						continue
					}
					idx.convert(value, iface)
				}
				return true
			})
		}
	}
}

// eachConversion calls f with each interface type module code converts
// concrete types to, and the concrete types converted.
func (idx *index) eachConversion(f func(iface types.Type, concretes []types.Type)) {
	idx.conversions.Iterate(func(iface types.Type, value any) {
		concretes, _ := value.([]types.Type)
		f(iface, concretes)
	})
}

// implementations are the concrete types module code converts to iface.
func (idx *index) implementations(iface types.Type) []types.Type {
	known, _ := idx.conversions.At(iface).([]types.Type)
	return known
}

// fieldOf returns the declaration of field i of the struct a pointer or value
// of type t holds.
func fieldOf(t types.Type, i int) *types.Var {
	if pointer, ok := t.Underlying().(*types.Pointer); ok {
		t = pointer.Elem()
	}
	st, _ := t.Underlying().(*types.Struct)
	return st.Field(i).Origin()
}

// loadedField returns the field v was loaded from, or nil.
func loadedField(v ssa.Value) *types.Var {
	switch v := v.(type) {
	case *ssa.UnOp:
		if addr, ok := v.X.(*ssa.FieldAddr); ok && v.Op == token.MUL {
			return fieldOf(addr.X.Type(), addr.Field)
		}
	case *ssa.Field:
		return fieldOf(v.X.Type(), v.Field)
	}
	return nil
}

// escapesField reports whether a field's address is put to a use other than
// a load, a store into it, or the address of one of its own fields.
func escapesField(addr *ssa.FieldAddr) bool {
	for _, ref := range *addr.Referrers() {
		switch ref := ref.(type) {
		case *ssa.UnOp:
			if ref.Op != token.MUL {
				return true
			}
		case *ssa.Store:
			if ref.Addr != addr {
				return true
			}
		case *ssa.FieldAddr, *ssa.IndexAddr, *ssa.DebugRef:
		default:
			return true
		}
	}
	return false
}
