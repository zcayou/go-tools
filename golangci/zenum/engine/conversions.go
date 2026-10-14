package engine

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// conversion reports a conversion into an enum, outside its declaring
// package, of anything but a constant holding a member's value or zero. Kind(n)
// makes a value no declaration names; parsing a token or decoding a number into
// a member belongs to the package that declares the members.
func (a *analysis) conversion(pkg *packages.Package, call *ast.CallExpr, to types.Type) {
	v := a.enumOf(to)
	if v == nil || v.pkg.Types == pkg.Types || len(call.Args) != 1 {
		return
	}
	// A constant operand is recorded with the type it is converted to, so only
	// a computed one can be told to be the enum's already.
	value := pkg.TypesInfo.Types[call].Value
	if value == nil && types.Identical(pkg.TypesInfo.TypeOf(call.Args[0]), v.Type) {
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
	if v == nil || v.pkg.Types == pkg.Types {
		return
	}
	switch expr := ast.Unparen(expr).(type) {
	case *ast.Ident:
		if typedConstant(info, expr, v) {
			return
		}
	case *ast.SelectorExpr:
		if typedConstant(info, expr.Sel, v) {
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
func typedConstant(info *types.Info, id *ast.Ident, v *enumType) bool {
	named, ok := info.Uses[id].(*types.Const)
	return ok && types.Identical(named.Type(), v.Type)
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

// genericConversion is a conversion to a type parameter, written in a generic
// body.
type genericConversion struct {
	call *ast.CallExpr
	pkg  *packages.Package
}

// instantiated reports a conversion to a type parameter that an instantiation
// makes a conversion into an enum, outside the enum's package:
// Convert[kinds.Kind](n) converting n with T(n). The instantiated body
// is where the type argument is known, and its conversion instruction carries
// the position of the conversion written in the generic body. A constant
// operand folds away in the instantiation, so only a computed one is seen.
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
				if !ok {
					continue
				}
				v := a.enumOf(to.Type())
				if v == nil || v.pkg.Types == written.pkg.Types || types.Identical(from.Type(), v.Type) {
					continue
				}
				a.computed(written.call.Pos(), v)
			}
		}
	}
}
