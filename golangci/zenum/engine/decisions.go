package engine

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/zcayou/go-tools/golangci/internal/decision"
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// renderings reports text decisions on a rendered token: v.String(), string(v)
// of a string enum, and fmt.Sprint of an enum value. A token is a spelling
// for readers, and renaming one must not change what the program does.
func (a *analysis) renderings(functions []*ssa.Function) {
	for _, fn := range functions {
		for _, found := range decision.Find(fn, func(value ssa.Value) bool { return a.rendered(value) != nil }) {
			v := a.rendered(found.Subject)
			a.report(found.Pos, RuleTokenDecision, v, fmt.Sprintf(
				"decides on %s's rendered token; compare the value with its members instead", v.display()))
		}
	}
}

// rendered returns the enum whose token value renders, or nil when value is no
// rendering.
func (a *analysis) rendered(value ssa.Value) *enumType {
	switch value := value.(type) {
	case *ssa.Call:
		callee := value.Call.StaticCallee()
		if callee == nil {
			return nil
		}
		// A wrapper promoting an embedded enum's String carries the method
		// it promotes as its object, and renders the same token.
		if method, ok := callee.Object().(*types.Func); ok && a.byName[method] != nil {
			return a.byName[method]
		}
		if obj := callee.Object(); obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "fmt" || obj.Name() != "Sprint" {
			return nil
		}
		args, _ := wholemodule.VariadicArgs(&value.Call)
		for _, arg := range args {
			if conversion, ok := arg.(*ssa.MakeInterface); ok {
				if v := a.enumOf(conversion.X.Type()); v != nil {
					return v
				}
			}
		}
	case *ssa.ChangeType:
		return a.textual(value.X.Type(), value.Type())
	case *ssa.Convert:
		return a.textual(value.X.Type(), value.Type())
	}
	return nil
}

// textual returns the string enum a conversion from one type to another
// renders as text, or nil.
func (a *analysis) textual(from, to types.Type) *enumType {
	v := a.enumOf(from)
	if v == nil || !v.Textual() || types.Identical(from, to) {
		return nil
	}
	if basic, ok := to.Underlying().(*types.Basic); !ok || basic.Info()&types.IsString == 0 {
		return nil
	}
	return v
}

// comparison judges an enum value compared with a constant, which pos
// places, at the expression the constant is written as. A member named,
// under its own name or a re-exported one, is what the comparison should say.
// Any other spelling of zero is a presence check, unless zero is a member:
// then it is that member, spelled as a number. Any other spelling is fine only
// while some member holds the value.
func (a *analysis) comparison(info *types.Info, v *enumType, expr ast.Expr, pos token.Pos) {
	value := info.Types[expr].Value
	if a.namesMember(info, v, expr) {
		return
	}
	if isZero(value) {
		if zero := v.zero(); zero != nil {
			a.report(pos, RuleTokenDecision, v, fmt.Sprintf(
				"compares %s with zero, which is its member %s; compare with the member by name",
				v.display(), zero.Name()))
		}
		return
	}
	if v.Member(value) {
		return
	}
	a.report(pos, RuleTokenDecision, v, fmt.Sprintf(
		"compares %s with %s, which no member holds", v.display(), value.ExactString()))
}

// namesMember reports whether expr names a constant of the enum that holds
// a member's value: a member, or a constant re-exporting one.
func (a *analysis) namesMember(info *types.Info, v *enumType, expr ast.Expr) bool {
	var id *ast.Ident
	switch expr := ast.Unparen(expr).(type) {
	case *ast.Ident:
		id = expr
	case *ast.SelectorExpr:
		id = expr.Sel
	default:
		return false
	}
	named, ok := info.Uses[id].(*types.Const)
	return ok && types.Identical(named.Type(), v.Type) && v.Member(named.Val())
}

// compared returns the enum of a comparison's non-constant operand x,
// when y is a constant.
func (a *analysis) compared(info *types.Info, x, y ast.Expr) *enumType {
	if info.Types[x].Value != nil || info.Types[y].Value == nil {
		return nil
	}
	return a.enumOf(info.TypeOf(x))
}

// switchCases judges each constant case of a switch on an enum value.
func (a *analysis) switchCases(info *types.Info, stmt *ast.SwitchStmt) {
	if stmt.Tag == nil || info.Types[stmt.Tag].Value != nil {
		return
	}
	v := a.enumOf(info.TypeOf(stmt.Tag))
	if v == nil {
		return
	}
	for _, clause := range stmt.Body.List {
		for _, expr := range clause.(*ast.CaseClause).List {
			if info.Types[expr].Value != nil {
				a.comparison(info, v, expr, expr.Pos())
			}
		}
	}
}

// binary judges an equality comparison of an enum value with a constant.
func (a *analysis) binary(info *types.Info, expr *ast.BinaryExpr) {
	if expr.Op != token.EQL && expr.Op != token.NEQ {
		return
	}
	if v := a.compared(info, expr.X, expr.Y); v != nil {
		a.comparison(info, v, expr.Y, expr.OpPos)
		return
	}
	if v := a.compared(info, expr.Y, expr.X); v != nil {
		a.comparison(info, v, expr.X, expr.OpPos)
	}
}
