package engine

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
)

// carriage answers which named types a value of a given type carries:
// the type itself, and what its pointers, slices, arrays, maps and channels
// hold, and the exported and embedded fields of the structs among them.
// An unexported field is the package's own state, and a function or interface
// type holds no value of anything until it is called or asserted.
type carriage struct {
	memo typeutil.Map // types.Type → []*types.Named
}

func newCarriage() *carriage {
	return &carriage{}
}

func (c *carriage) of(t types.Type) []*types.Named {
	if found, ok := c.memo.At(t).([]*types.Named); ok {
		return found
	}
	var found []*types.Named
	var visited typeutil.Map
	var walk func(types.Type)
	walk = func(t types.Type) {
		t = types.Unalias(t)
		if visited.At(t) != nil {
			return
		}
		visited.Set(t, true)
		switch t := t.(type) {
		case *types.Named:
			found = append(found, t)
			walk(t.Underlying())
		case *types.Pointer:
			walk(t.Elem())
		case *types.Slice:
			walk(t.Elem())
		case *types.Array:
			walk(t.Elem())
		case *types.Chan:
			walk(t.Elem())
		case *types.Map:
			walk(t.Key())
			walk(t.Elem())
		case *types.Struct:
			for field := range t.Fields() {
				if field.Exported() || field.Embedded() {
					walk(field.Type())
				}
			}
		}
	}
	walk(t)
	c.memo.Set(t, found)
	return found
}

// see decides which enums are visible: those a reader outside their package can
// meet. An enum is visible when it is exported, when it is an error type, when
// an exported declaration exposes it, and when module code converts a value
// carrying it to an interface — fmt and any parameter included, since a value
// behind an interface reaches whatever the interface is handed to.
//
// Two conversions are not counted. One inside a method of the enum's own
// type is its own business, such as its String printing a value no member holds
// with fmt. One handed only to an opaque sink, which never renders it, reaches
// no reader.
func (a *analysis) see(functions []*ssa.Function) {
	met := a.exposed()
	for _, fn := range functions {
		own := a.ownerOf(fn)
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				conversion, ok := instr.(*ssa.MakeInterface)
				if !ok || opaque(conversion) {
					continue
				}
				for _, named := range a.carriage.of(conversion.X.Type()) {
					if own == nil || named != own.Type {
						met[named] = true
					}
				}
			}
		}
	}

	errorType, _ := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	for _, v := range a.enums {
		v.visible = v.Type.Obj().Exported() || met[v.Type] ||
			types.Implements(v.Type, errorType) || types.Implements(types.NewPointer(v.Type), errorType)
	}
}

// ownerOf returns the enum whose method fn is written in, closures included,
// or nil.
func (a *analysis) ownerOf(fn *ssa.Function) *enumType {
	top := fn
	for top.Parent() != nil {
		top = top.Parent()
	}
	if origin := top.Origin(); origin != nil {
		top = origin
	}
	return a.receiverEnum(top.Signature)
}

// opaqueSinks are the standard-library functions and methods,
// by [types.Func.FullName], that take a value as an interface and never render
// it: they key, store, sort or describe the type of what they are given.
// A value handed to one meets no reader through it.
var opaqueSinks = map[string]bool{
	"context.WithValue":                   true,
	"(context.Context).Value":             true,
	"(*sync.Map).CompareAndDelete":        true,
	"(*sync.Map).CompareAndSwap":          true,
	"(*sync.Map).Delete":                  true,
	"(*sync.Map).Load":                    true,
	"(*sync.Map).LoadAndDelete":           true,
	"(*sync.Map).LoadOrStore":             true,
	"(*sync.Map).Store":                   true,
	"(*sync.Map).Swap":                    true,
	"(*sync/atomic.Value).CompareAndSwap": true,
	"(*sync/atomic.Value).Store":          true,
	"(*sync/atomic.Value).Swap":           true,
	"sort.Slice":                          true,
	"sort.SliceIsSorted":                  true,
	"sort.SliceStable":                    true,
	"reflect.TypeOf":                      true,
}

// opaque reports whether a conversion's interface is only ever handed
// to opaque sinks, as an argument.
func opaque(conversion *ssa.MakeInterface) bool {
	refs := conversion.Referrers()
	if refs == nil || len(*refs) == 0 {
		return false
	}
	for _, ref := range *refs {
		call, ok := ref.(ssa.CallInstruction)
		if !ok || call.Common().Value == conversion || !opaqueSinks[calleeName(call.Common())] {
			return false
		}
	}
	return true
}

// calleeName is the full name of the function or interface method a call
// statically names, or "".
func calleeName(call *ssa.CallCommon) string {
	if call.IsInvoke() {
		return call.Method.FullName()
	}
	if callee := call.StaticCallee(); callee != nil {
		if fn, ok := callee.Object().(*types.Func); ok {
			return fn.FullName()
		}
	}
	return ""
}

// exposed returns the named types exported declarations expose: the type
// of an exported variable or constant, a parameter or result of an exported
// function, and what an exposed type carries. The exported methods of a type
// once exposed expose their own parameters and results, including the methods
// an exposed interface declares, so an unexported type an exported function
// returns publishes its methods' types as an exported one does.
func (a *analysis) exposed() map[*types.Named]bool {
	exposed := map[*types.Named]bool{}
	var queue []*types.Named
	expose := func(t types.Type) {
		for _, named := range a.carriage.of(t) {
			if !exposed[named] {
				exposed[named] = true
				queue = append(queue, named)
			}
		}
	}
	exposeSignature := func(sig *types.Signature) {
		for param := range sig.Params().Variables() {
			expose(param.Type())
		}
		for result := range sig.Results().Variables() {
			expose(result.Type())
		}
	}

	for _, pkg := range a.module.Packages {
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if !obj.Exported() {
				continue
			}
			switch obj := obj.(type) {
			case *types.Func:
				exposeSignature(obj.Signature())
			case *types.TypeName, *types.Var, *types.Const:
				expose(obj.Type())
			}
		}
	}

	for len(queue) > 0 {
		named := queue[0]
		queue = queue[1:]
		for method := range named.Methods() {
			if method.Exported() {
				exposeSignature(method.Signature())
			}
		}
		if iface, ok := named.Underlying().(*types.Interface); ok {
			for method := range iface.Methods() {
				if method.Exported() {
					exposeSignature(method.Signature())
				}
			}
		}
	}
	return exposed
}

// functions returns every SSA function holding module code: each function
// and method the module's syntax declares, the package initializers, which hold
// package-level variables' initialization, the methods of each module type
// the program converts to an interface, and what those reach that is still
// module code — closures, and the instantiations of generic bodies the module
// builds.
//
// A method reached only through an interface is called by no instruction,
// so the program's runtime types are what find it, an instantiated method
// of a generic type above all. Building such a method on demand is safe while
// the other linters reading the same load walk the program: [ssa.Program]
// guards it. A method with type parameters of its own has no method value
// to build, and is read only through the instantiations its callers make.
func (a *analysis) functions() []*ssa.Function {
	prog := a.module.Program()
	module := map[*ssa.Package]bool{}
	for _, pkg := range prog.Packages {
		module[pkg] = true
	}

	seen := map[*ssa.Function]bool{}
	var found []*ssa.Function
	var visit func(*ssa.Function)
	visit = func(fn *ssa.Function) {
		if fn == nil || seen[fn] || !moduleCode(fn, module) {
			return
		}
		seen[fn] = true
		found = append(found, fn)
		for _, anon := range fn.AnonFuncs {
			visit(anon)
		}
		var operands []*ssa.Value
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				operands = instr.Operands(operands[:0])
				for _, operand := range operands {
					if callee, ok := (*operand).(*ssa.Function); ok {
						visit(callee)
					}
				}
			}
		}
	}

	for _, pkg := range a.module.Packages {
		ssaPkg := prog.Packages[pkg.Types]
		visit(ssaPkg.Func("init"))
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if fn, isFunc := decl.(*ast.FuncDecl); isFunc {
					if obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func); ok {
						visit(prog.FuncValue(obj))
					}
				}
			}
		}
	}
	for _, t := range prog.RuntimeTypes() {
		if !a.moduleType(t) {
			continue
		}
		for selection := range types.NewMethodSet(t).Methods() {
			if method, ok := selection.Obj().(*types.Func); ok && method.Signature().TypeParams().Len() > 0 {
				continue
			}
			visit(prog.MethodValue(selection))
		}
	}
	return found
}

// moduleType reports whether t is a defined type a module package declares,
// or a pointer to one, other than an interface.
func (a *analysis) moduleType(t types.Type) bool {
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || types.IsInterface(named) {
		return false
	}
	return a.module.Declares(named.Obj())
}

// moduleCode reports whether fn's body is module code: whether the function
// it is written in, or the generic it instantiates, belongs to a module
// package. Synthesized wrappers belong to none.
func moduleCode(fn *ssa.Function, module map[*ssa.Package]bool) bool {
	top := fn
	for top.Parent() != nil {
		top = top.Parent()
	}
	if origin := top.Origin(); origin != nil {
		top = origin
	}
	return top.Pkg != nil && module[top.Pkg]
}
