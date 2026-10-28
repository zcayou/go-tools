package engine

import (
	"fmt"
	"go/ast"
	"go/constant"
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
	a.ordered(pkg, pkg.TypesInfo.TypeOf(expr.X), expr.OpPos, expr.X, expr.Y)
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
	a.ordered(pkg, pkg.TypesInfo.TypeOf(call), call.Pos(), call.Args...)
}

// ordered judges an ordering of an ordinal enum's values, written on the enum's
// own type, unless ordered-enums lists it. Most enums are unordered sets
// that happen to be numbered: State > Running assumes declaration order
// is lifecycle order, and silently admits whatever member is inserted between
// them. The declaring package owns what its ordinals mean, so outside it every
// ordering is reported, and inside it only a bound by the highest member.
// An ordering of a type parameter is not judged here; a sort instantiated
// with the enum is judged by instantiation, when it is judged at all.
func (a *analysis) ordered(pkg *packages.Package, t types.Type, pos token.Pos, operands ...ast.Expr) {
	v := a.enumOf(t)
	if v == nil || !v.Ordinal() {
		return
	}
	if v.inside(pkg) {
		a.bound(pkg.TypesInfo, v, pos, operands)
		return
	}
	if v.orderedEnum {
		a.excuse(v, RuleOrderedComparison)
		return
	}
	a.report(pos, RuleOrderedComparison, v, fmt.Sprintf(
		"orders %s, whose ordinals are an encoding rather than an order; decide membership in %s, "+
			"or list it in ordered-enums", v.display(), v.pkg.PkgPath))
}

// bound reports an ordering of the enum's values against a constant holding
// its highest member's value, as in k <= KindC. It keeps compiling when
// a member is appended, and silently leaves the new member out. A count
// sentinel is no member, and is the bound that moves with the members.
func (a *analysis) bound(info *types.Info, v *enumType, pos token.Pos, operands []ast.Expr) {
	top := v.highest()
	computed, bounded := false, false
	for _, operand := range operands {
		switch value := info.Types[operand].Value; {
		case value == nil:
			computed = true
		case constant.Compare(value, token.EQL, top.Val()):
			bounded = true
		}
	}
	if !computed || !bounded {
		return
	}
	if v.orderedEnum {
		a.excuse(v, RuleMemberBound)
		return
	}
	a.report(pos, RuleMemberBound, v, fmt.Sprintf(
		"bounds %s by %s, its highest member, so a member added after %s falls outside; "+
			"bound it by a count sentinel, or switch over the members",
		v.Type.Obj().Name(), top.Name(), top.Name()))
}

// instantiation judges an instantiation of a generic function, method or type,
// written at id, that passes an ordinal enum outside its package to a type
// parameter whose constraint orders its values, as cmp.Ordered does:
// slices.Sort over the enum's values orders them by ordinal exactly as < does.
func (a *analysis) instantiation(pkg *packages.Package, id *ast.Ident) {
	instance, ok := pkg.TypesInfo.Instances[id]
	if !ok {
		return
	}
	generic := pkg.TypesInfo.Uses[id]
	params := typeParams(generic)
	if params == nil || params.Len() != instance.TypeArgs.Len() {
		return
	}
	for i := range params.Len() {
		v := a.enumOf(instance.TypeArgs.At(i))
		if v == nil || !v.Ordinal() || v.inside(pkg) || !orders(params.At(i).Constraint()) {
			continue
		}
		if v.orderedEnum {
			a.excuse(v, RuleOrderedComparison)
			continue
		}
		a.report(id.Pos(), RuleOrderedComparison, v, fmt.Sprintf(
			"orders %s through %s, whose ordinals are an encoding rather than an order; order by its tokens, "+
				"or list it in ordered-enums", v.display(), genericName(generic)))
	}
}

// typeParams returns the type parameters of a generic function, method or type,
// or nil.
func typeParams(obj types.Object) *types.TypeParamList {
	switch obj := obj.(type) {
	case *types.Func:
		return obj.Origin().Signature().TypeParams()
	case *types.TypeName:
		switch t := obj.Type().(type) {
		case *types.Named:
			return t.Origin().TypeParams()
		case *types.Alias:
			return t.Origin().TypeParams()
		}
	}
	return nil
}

// genericName writes a generic as source code outside its package names it:
// its package's name, its receiver type's for a method, and its own.
func genericName(obj types.Object) string {
	name := obj.Name()
	if fn, ok := obj.(*types.Func); ok && fn.Signature().Recv() != nil {
		recv := fn.Signature().Recv().Type()
		if pointer, ok := recv.(*types.Pointer); ok {
			recv = pointer.Elem()
		}
		if named, ok := types.Unalias(recv).(*types.Named); ok {
			name = named.Obj().Name() + "." + name
		}
	}
	return obj.Pkg().Name() + "." + name
}

// orders reports whether a constraint permits ordering its type parameter's
// values: whether every type in its type set is ordered. An interface's type
// set is the intersection of its elements', so one element holding only
// ordered types is enough; a union holds only ordered types when each of its
// terms does.
func orders(constraint types.Type) bool {
	iface, ok := constraint.Underlying().(*types.Interface)
	if !ok {
		return ordinary(constraint)
	}
	for element := range iface.EmbeddedTypes() {
		if union, ok := element.(*types.Union); ok {
			if allOrdered(union) {
				return true
			}
			continue
		}
		if orders(element) {
			return true
		}
	}
	return false
}

// allOrdered reports whether every term of a union holds only ordered types.
func allOrdered(union *types.Union) bool {
	for term := range union.Terms() {
		if !orders(term.Type()) {
			return false
		}
	}
	return union.Len() > 0
}

// ordinary reports whether a type that is no interface is ordered.
func ordinary(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsOrdered != 0
}

// unordered returns an error for each ordered-enums entry that suppressed
// no ordering a running rule judges.
func (a *analysis) unordered(cfg Config) []error {
	var stale []error
	for _, name := range cfg.OrderedEnums {
		if v := a.lookup(name); v != nil && !v.excused {
			stale = append(stale, &ExceptionError{Setting: SettingOrderedEnums, Fault: FaultNotOrdered, Type: name})
		}
	}
	return stale
}
