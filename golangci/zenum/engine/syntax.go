package engine

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
)

// syntax runs the rules read from source rather than from SSA. Each turns
// on how a constant is spelled, or on whether a conversion is written at all,
// and SSA keeps neither: a member's name and the number it stands for are one
// constant there, and a conversion of a constant folds away.
func (a *analysis) syntax() {
	for _, pkg := range a.module.Packages {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				r := &reading{
					a: a, pkg: pkg, own: a.owner(pkg.TypesInfo, decl),
					skip: map[ast.Expr]bool{}, stored: map[ast.Expr]bool{},
				}
				ast.Inspect(decl, r.visit)
			}
		}
	}
}

// reading is one walk of a declaration's syntax.
type reading struct {
	a   *analysis
	pkg *packages.Package

	// own is the enum the declaration is a method of, or nil.
	own *enumType

	// skip holds the constants a comparison, a case or an index lookup is written
	// with, which token-decision judges or which make no value.
	skip map[ast.Expr]bool

	// stored holds the operands handed directly to a sync/atomic integer's
	// write, which keeps a conversion's ordinal without reading it.
	stored map[ast.Expr]bool
}

func (r *reading) visit(node ast.Node) bool {
	info := r.pkg.TypesInfo
	switch node := node.(type) {
	case *ast.BinaryExpr:
		r.a.binary(info, node)
		r.a.ordering(r.pkg, node)
		if comparison(node.Op) {
			r.skip[node.X], r.skip[node.Y] = true, true
		}
	case *ast.SwitchStmt:
		r.a.switchCases(info, node)
		if node.Tag != nil {
			for _, clause := range node.Body.List {
				for _, expr := range clause.(*ast.CaseClause).List {
					r.skip[expr] = true
				}
			}
		}
	case *ast.IndexExpr:
		r.skip[node.Index] = true
	case *ast.CallExpr:
		r.call(node)
	}
	if expr, ok := node.(ast.Expr); ok && info.Types[expr].Value != nil {
		if !r.skip[expr] {
			r.a.constant(r.pkg, expr)
		}
		// A constant's parts are folded into it, and judged as the whole.
		return false
	}
	return true
}

// call judges a call: a conversion, or a call of a function that may format
// its operands, order them, or store them atomically.
func (r *reading) call(call *ast.CallExpr) {
	info := r.pkg.TypesInfo
	if tv, ok := info.Types[call.Fun]; ok && tv.IsType() {
		r.a.conversion(r.pkg, call, tv.Type)
		if !r.stored[call] {
			r.a.laundered(r.pkg, call, tv.Type)
		}
		if typeParam(tv.Type) || len(call.Args) == 1 && typeParam(info.TypeOf(call.Args[0])) {
			r.a.generic[call.Lparen] = genericConversion{call: call, pkg: r.pkg, stored: r.stored[call]}
		}
		return
	}
	r.a.formatting(info, call, r.own)
	r.a.extreme(r.pkg, call)
	if callee, ok := typeutil.Callee(info, call).(*types.Func); ok && atomicWrite(callee) {
		for _, arg := range call.Args {
			r.stored[ast.Unparen(arg)] = true
		}
	}
}

// owner returns the enum decl is a method of, or nil.
func (a *analysis) owner(info *types.Info, decl ast.Decl) *enumType {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok || fn.Recv == nil {
		return nil
	}
	method, ok := info.Defs[fn.Name].(*types.Func)
	if !ok {
		return nil
	}
	return a.receiverEnum(method.Signature())
}

// receiverEnum returns the enum a method signature's receiver is, through
// a pointer, or nil.
func (a *analysis) receiverEnum(sig *types.Signature) *enumType {
	if sig.Recv() == nil {
		return nil
	}
	recv := sig.Recv().Type()
	if pointer, ok := recv.(*types.Pointer); ok {
		recv = pointer.Elem()
	}
	return a.enumOf(recv)
}

// comparison reports whether op compares its operands.
func comparison(op token.Token) bool {
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	default:
		return false
	}
}

// typeParam reports whether t is a type parameter.
func typeParam(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.TypeParam)
	return ok
}
