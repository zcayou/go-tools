package engine

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// ordering judges a comparison ordering its operands: <, <=, > and >=.
func (a *analysis) ordering(pkg *packages.Package, expr *ast.BinaryExpr) {
	switch expr.Op {
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
	default:
		return
	}
	a.ordered(pkg, pkg.TypesInfo.TypeOf(expr.X), expr.OpPos)
}

// extreme judges a call of the min or max builtin.
func (a *analysis) extreme(pkg *packages.Package, call *ast.CallExpr) {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return
	}
	builtin, ok := pkg.TypesInfo.Uses[id].(*types.Builtin)
	if !ok || builtin.Name() != "min" && builtin.Name() != "max" {
		return
	}
	a.ordered(pkg, pkg.TypesInfo.TypeOf(call), call.Pos())
}

// ordered reports an ordering of an ordinal enum's values, written
// on the enum's own type outside its package, unless ordered-enums lists it.
// Most enums are unordered sets that happen to be numbered: State > Running
// assumes declaration order is lifecycle order, and silently admits whatever
// member is inserted between them. The declaring package owns what its ordinals
// mean. An ordering of a type parameter is not judged, so neither is a sort
// instantiated with the enum: sorting by ordinal decides no membership.
func (a *analysis) ordered(pkg *packages.Package, t types.Type, pos token.Pos) {
	v := a.enumOf(t)
	if v == nil || !v.Ordinal() || v.pkg.Types == pkg.Types {
		return
	}
	if v.orderedEnum {
		v.orderedOutside = true
		return
	}
	a.report(pos, RuleOrderedComparison, v, fmt.Sprintf(
		"orders %s, whose ordinals are an encoding rather than an order; decide membership in %s, "+
			"or list it in ordered-enums", v.display(), v.pkg.PkgPath))
}

// unordered returns an error for each ordered-enums entry that nothing outside
// its package orders.
func (a *analysis) unordered(cfg Config) []error {
	var stale []error
	for _, name := range cfg.OrderedEnums {
		if v := a.lookup(name); v != nil && !v.orderedOutside {
			stale = append(stale, &ExceptionError{Setting: SettingOrderedEnums, Fault: FaultNotOrdered, Type: name})
		}
	}
	return stale
}
