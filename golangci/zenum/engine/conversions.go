package engine

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// conversion reports a conversion into an enum, outside its declaring
// package, of anything but a constant holding a member's value or zero. Kind(n)
// makes a value no declaration names; parsing a token or decoding a number into
// a member belongs to the package that declares the members.
func (a *analysis) conversion(pkg *packages.Package, call *ast.CallExpr, to types.Type) {
	v := a.enumOf(to)
	if v == nil || v.inside(pkg) || len(call.Args) != 1 {
		return
	}
	// A constant operand is recorded with the type it is converted to, so only
	// a computed one can be told to be the enum's already.
	value := pkg.TypesInfo.Types[call].Value
	if value == nil && a.enumOf(pkg.TypesInfo.TypeOf(call.Args[0])) == v {
		return
	}
	if value == nil {
		a.computed(call.Pos(), v)
		return
	}
	a.forged(call.Pos(), v, value)
}

// constant reports an untyped constant given an enum's type outside its
// declaring package, which is a conversion the source does not write:
// an assignment, an argument, a declared value, or arithmetic on a member,
// as in Take(42) or KindB + 5. A constant declared with the enum's type
// is judged where it is declared rather than at each use, and a written
// conversion by conversion.
func (a *analysis) constant(pkg *packages.Package, expr ast.Expr) {
	info := pkg.TypesInfo
	v := a.enumOf(info.TypeOf(expr))
	if v == nil || v.inside(pkg) {
		return
	}
	switch expr := ast.Unparen(expr).(type) {
	case *ast.Ident:
		if a.typedConstant(info, expr, v) {
			return
		}
	case *ast.SelectorExpr:
		if a.typedConstant(info, expr.Sel, v) {
			return
		}
	case *ast.CallExpr:
		if tv, ok := info.Types[expr.Fun]; ok && tv.IsType() {
			return
		}
	}
	a.forged(expr.Pos(), v, info.Types[expr].Value)
}

// typedConstant reports whether id names a constant declared with the enum's
// type, which is judged where it is declared.
func (a *analysis) typedConstant(info *types.Info, id *ast.Ident, v *enumType) bool {
	named, ok := info.Uses[id].(*types.Const)
	return ok && a.enumOf(named.Type()) == v
}

// computed reports a computed value converted to an enum outside its package.
func (a *analysis) computed(pos token.Pos, v *enumType) {
	a.report(pos, RuleForgedConversion, v, fmt.Sprintf(
		"converts a computed value to %s outside its package; only %s's package may make a member from one",
		v.display(), v.pkg.Name))
}

// forged reports a constant given an enum's type outside its package,
// unless a member holds its value or it is zero.
func (a *analysis) forged(pos token.Pos, v *enumType, value constant.Value) {
	if isZero(value) || v.Member(value) {
		return
	}
	a.report(pos, RuleForgedConversion, v, fmt.Sprintf(
		"converts %s, which no member holds, to %s", value.ExactString(), v.display()))
}

// genericConversion is a conversion to or from a type parameter, written
// in a generic body.
type genericConversion struct {
	call *ast.CallExpr
	pkg  *packages.Package

	// stored reports that the conversion is handed directly to a sync/atomic
	// integer's write.
	stored bool
}

// instantiated judges a conversion to or from a type parameter in each
// instantiation the module makes. It reports one that an instantiation makes
// a conversion into an enum, outside the enum's package: Convert[kinds.Kind](n)
// converting n with T(n), and one it makes a conversion out of an enum
// that laundered judges. The instantiated body is where the type argument
// is known, and its conversion instruction carries the position
// of the conversion written in the generic body. A constant operand folds away
// in the instantiation, so only a computed one is seen.
func (a *analysis) instantiated(functions []*ssa.Function) {
	for _, fn := range functions {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				var from, to ssa.Value
				switch instr := instr.(type) {
				case *ssa.ChangeType:
					from, to = instr.X, instr
				case *ssa.Convert:
					from, to = instr.X, instr
				default:
					continue
				}
				written, ok := a.generic[to.Pos()]
				if !ok || typeParam(from.Type()) || typeParam(to.Type()) {
					continue
				}
				if !written.stored {
					a.launder(written.pkg, written.call.Pos(), from.Type(), to.Type())
				}
				v := a.enumOf(to.Type())
				if v == nil || v.inside(written.pkg) || a.enumOf(from.Type()) == v {
					continue
				}
				a.computed(written.call.Pos(), v)
			}
		}
	}
}

// laundered reports a conversion of an ordinal enum's value to any other
// non-interface type, outside its package, unless the target is another enum,
// which conversion judges. int(kind) keeps the member's ordinal and drops its
// type: the number travels into a row, a label or a key that nothing ties back
// to the declaration assigning it, and that changes when a member is inserted.
// The declaring package owns its encoding in both directions. A conversion
// to or from a type parameter is judged in each instantiation instead.
func (a *analysis) laundered(pkg *packages.Package, call *ast.CallExpr, to types.Type) {
	if len(call.Args) != 1 || typeParam(to) {
		return
	}
	a.launder(pkg, call.Pos(), pkg.TypesInfo.TypeOf(call.Args[0]), to)
}

// launder reports a conversion from a value of type from to type to, written
// at pos in pkg, that keeps an enum's ordinal and drops its type.
func (a *analysis) launder(pkg *packages.Package, pos token.Pos, from, to types.Type) {
	v := a.enumOf(from)
	if v == nil || !v.Ordinal() || v.inside(pkg) ||
		types.Identical(from, to) || types.IsInterface(to) || a.enumOf(to) != nil {
		return
	}
	a.report(pos, RuleLaunderedConversion, v, fmt.Sprintf(
		"converts %s to %s, which keeps its ordinal and drops its type; use its String, or convert in %s",
		v.display(), types.TypeString(to, a.qualifier), v.pkg.PkgPath))
}

// atomicIntegers and atomicWrites are the sync/atomic integer types,
// and the operations writing one: a method of the type, or a function named
// for the operation and the type.
var (
	atomicIntegers = []string{"Int32", "Int64", "Uint32", "Uint64", "Uintptr"}
	atomicWrites   = []string{"Store", "Swap", "CompareAndSwap", "Add"}
)

// atomicWrite reports whether fn writes a sync/atomic integer, which keeps
// an ordinal without reading it: it comes back a member only through
// a conversion into the enum, which conversion judges.
func atomicWrite(fn *types.Func) bool {
	if fn.Pkg() == nil || fn.Pkg().Path() != "sync/atomic" {
		return false
	}
	if recv := fn.Signature().Recv(); recv != nil {
		t := recv.Type()
		if pointer, ok := t.(*types.Pointer); ok {
			t = pointer.Elem()
		}
		named, ok := types.Unalias(t).(*types.Named)
		return ok && slices.Contains(atomicIntegers, named.Obj().Name()) && slices.Contains(atomicWrites, fn.Name())
	}
	for _, write := range atomicWrites {
		if integer, ok := strings.CutPrefix(fn.Name(), write); ok && slices.Contains(atomicIntegers, integer) {
			return true
		}
	}
	return false
}
