package engine

import (
	"go/types"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/zcayou/go-tools/golangci/internal/enum"
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// reached is an ordinal enum fmt formats when it formats an operand, and what
// keeps fmt from calling its String there.
type reached struct {
	enum *enumType

	// field is the outermost unexported field the enum sits beneath, written
	// Owner.field, or empty when fmt can call its methods. fmt cannot call
	// a method on what it cannot turn back into an interface.
	field string

	// bytes is the slice or array type holding the enum as elements of uint8
	// kind, which fmt prints as raw bytes under %s and %q, or nil.
	bytes types.Type
}

// reach returns the ordinal enums fmt formats under verb when it formats
// a value of type t. fmt applies the verb to everything it descends into:
// struct fields, exported or not, and embedded structs; slice and array
// elements; map keys and values; and a pointer's target at the top level only,
// deeper pointers being printed as addresses. It stops at a value it hands
// to the value's own method, which it can call only on what is not beneath
// an unexported field: a Formatter under every verb, and an error or Stringer
// under the verbs that call them, %v, %s, %q, %x and %X. A value of interface
// type is whatever it holds, which is not known statically.
func (a *analysis) reach(t types.Type, verb rune) []reached {
	w := &walker{a: a, verb: verb, found: map[reached]bool{}}
	w.walk(t, true, "")
	return slices.Collect(maps.Keys(w.found))
}

type walker struct {
	a     *analysis
	verb  rune
	seen  typeutil.Map // types.Type → map[string]bool, by field
	found map[reached]bool
}

func (w *walker) walk(t types.Type, top bool, field string) {
	t = types.Unalias(t)
	if v := w.a.enumOf(t); v != nil {
		if v.Ordinal() {
			w.enum(v, field)
		}
		return
	}
	if !top {
		fields, _ := w.seen.At(t).(map[string]bool)
		if fields[field] {
			return
		}
		if fields == nil {
			fields = map[string]bool{}
			w.seen.Set(t, fields)
		}
		fields[field] = true
	}
	if field == "" && w.handled(t) {
		return
	}

	switch u := t.Underlying().(type) {
	case *types.Struct:
		for f := range u.Fields() {
			next := field
			if next == "" && !f.Exported() {
				next = fieldName(t, f)
			}
			w.walk(f.Type(), false, next)
		}
	case *types.Slice:
		w.elements(t, u.Elem(), field)
	case *types.Array:
		w.elements(t, u.Elem(), field)
	case *types.Map:
		w.walk(u.Key(), false, field)
		w.walk(u.Elem(), false, field)
	case *types.Pointer:
		if !top {
			return
		}
		switch u.Elem().Underlying().(type) {
		case *types.Struct, *types.Array, *types.Slice, *types.Map:
			w.walk(u.Elem(), false, field)
		}
	}
}

// enum records an enum the walk reaches, when the verb formats it as no
// member's name.
func (w *walker) enum(v *enumType, field string) {
	switch {
	case strings.ContainsRune(bypassing, w.verb):
		w.found[reached{enum: v}] = true
	case field != "" && enum.Name(v.Type) != nil:
		w.found[reached{enum: v, field: field}] = true
	}
}

// elements walks a slice or array's elements, which fmt prints as raw bytes
// under %s and %q when their kind is uint8.
func (w *walker) elements(t, elem types.Type, field string) {
	if v := w.a.enumOf(elem); v != nil && v.Ordinal() && (w.verb == 's' || w.verb == 'q') &&
		enum.Name(v.Type) != nil {
		if basic, ok := elem.Underlying().(*types.Basic); ok && basic.Kind() == types.Uint8 {
			w.found[reached{enum: v, bytes: t}] = true
			return
		}
	}
	w.walk(elem, false, field)
}

// handled reports whether fmt hands a value of type t to the value's own
// method under the walk's verb.
func (w *walker) handled(t types.Type) bool {
	methods := types.NewMethodSet(t)
	if formatter(methods) {
		return true
	}
	if !strings.ContainsRune("vsqxX", w.verb) {
		return false
	}
	return nullary(methods, "Error") || nullary(methods, "String")
}

// formatter reports whether a method set has fmt.Formatter's Format.
func formatter(methods *types.MethodSet) bool {
	selection := methods.Lookup(nil, "Format")
	if selection == nil {
		return false
	}
	method, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	sig := method.Signature()
	if sig.Params().Len() != 2 || sig.Results().Len() != 0 {
		return false
	}
	state, ok := types.Unalias(sig.Params().At(0).Type()).(*types.Named)
	return ok && state.Obj().Pkg() != nil && state.Obj().Pkg().Path() == "fmt" && state.Obj().Name() == "State" &&
		types.Identical(sig.Params().At(1).Type(), types.Typ[types.Rune])
}

// nullary reports whether a method set has a method name() string, as error's
// Error and fmt.Stringer's String are.
func nullary(methods *types.MethodSet, name string) bool {
	selection := methods.Lookup(nil, name)
	if selection == nil {
		return false
	}
	method, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	sig := method.Signature()
	return sig.Params().Len() == 0 && sig.Results().Len() == 1 &&
		types.Identical(sig.Results().At(0).Type(), types.Typ[types.String])
}

// fieldName writes a field as its struct's name and its own, or as its own
// for a struct with no name.
func fieldName(owner types.Type, f *types.Var) string {
	if named, ok := owner.(*types.Named); ok {
		return named.Obj().Name() + "." + f.Name()
	}
	return f.Name()
}

// holdsTypeParam reports whether t holds a type parameter anywhere fmt could
// descend to.
func holdsTypeParam(t types.Type) bool {
	var seen typeutil.Map
	var holds func(types.Type) bool
	holds = func(t types.Type) bool {
		t = types.Unalias(t)
		if seen.At(t) != nil {
			return false
		}
		seen.Set(t, true)
		switch t := t.(type) {
		case *types.TypeParam:
			return true
		case *types.Named:
			for arg := range t.TypeArgs().Types() {
				if holds(arg) {
					return true
				}
			}
			return false
		case *types.Pointer:
			return holds(t.Elem())
		case *types.Slice:
			return holds(t.Elem())
		case *types.Array:
			return holds(t.Elem())
		case *types.Map:
			return holds(t.Key()) || holds(t.Elem())
		case *types.Struct:
			for f := range t.Fields() {
				if holds(f.Type()) {
					return true
				}
			}
		}
		return false
	}
	return holds(t)
}

// genericFormat is a printer's call in a generic body, and the operands
// it formats whose types hold a type parameter.
type genericFormat struct {
	operands []formatted
	own      *enumType
}

// instantiatedFormats judges each generic call recorded by the syntax walk
// in every instantiation the module makes, where the type arguments are known.
// The instantiated call carries the position of the call written in the generic
// body, and its operands are the conversions to any that pass them.
func (a *analysis) instantiatedFormats(functions []*ssa.Function) {
	for _, fn := range functions {
		if fn.Origin() == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(*ssa.Call)
				if !ok {
					continue
				}
				written, ok := a.genericFormats[call.Pos()]
				if !ok {
					continue
				}
				args, ok := wholemodule.VariadicArgs(call.Common())
				if !ok {
					continue
				}
				for _, operand := range written.operands {
					if operand.index >= len(args) {
						continue
					}
					if conversion, ok := args[operand.index].(*ssa.MakeInterface); ok {
						a.judgeFormatted(operand, conversion.X.Type(), written.own)
					}
				}
			}
		}
	}
}
