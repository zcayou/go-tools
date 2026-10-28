package engine

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
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

// opaque reports whether a conversion's interface is only ever handed
// to standard-library sinks, as an argument they hold or compare and never
// render.
func opaque(conversion *ssa.MakeInterface) bool {
	refs := conversion.Referrers()
	if refs == nil || len(*refs) == 0 {
		return false
	}
	for _, ref := range *refs {
		call, ok := ref.(ssa.CallInstruction)
		if !ok || wholemodule.SinkOf(call.Common(), conversion) == 0 {
			return false
		}
	}
	return true
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
