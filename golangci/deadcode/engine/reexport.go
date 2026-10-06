package engine

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// reexport is one copy on the declared surface: a declaration renaming one
// another surface package makes. It is held under the copy's own declaration
// key.
type reexport struct {
	// original is the declaration key of what the copy renames, itself possibly
	// a copy.
	original string
	// rhs is the copy's right-hand side, where its one reference to the original
	// defines the second name rather than using the first.
	rhs span
}

// reexports holds every copy on the declared surface. Whether a declaration
// is a copy is a fact about its source, the same in every view, so it is read
// once for the whole run.
type reexports map[string]reexport

// newReexports records the copies the declared surface makes: exported
// declarations whose whole definition names a declaration another surface
// package makes. Only a candidate can be one, so a copy in a test or generated
// file stays what it was.
func newReexports(pkgs []*packages.Package, facts map[*packages.Package]fileFacts, surface *apiSurface) reexports {
	copies := reexports{}
	for _, pkg := range pkgs {
		if synthesizedPackage(pkg.PkgPath) || !surface.packages[pkg.PkgPath] {
			continue
		}
		for _, file := range pkg.Syntax {
			name := position(pkg.Fset, file.Pos()).Filename
			if testFile(name) || facts[pkg].generated[name] {
				continue
			}
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.FuncDecl:
					copies.addForwarder(pkg, surface, decl)
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						switch spec := spec.(type) {
						case *ast.TypeSpec:
							copies.addTypeReexport(pkg, surface, spec)
						case *ast.ValueSpec:
							if decl.Tok == token.CONST {
								copies.addConstReexports(pkg, surface, spec)
							}
						}
					}
				}
			}
		}
	}
	return copies
}

// addTypeReexport records an alias of another package's named type, generic
// instantiations included: an alias is the type it names, so everything
// spelled through it spells the original.
func (r reexports) addTypeReexport(pkg *packages.Package, surface *apiSurface, spec *ast.TypeSpec) {
	obj, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName)
	if !ok || !obj.Exported() {
		return
	}
	alias, ok := obj.Type().(*types.Alias)
	if !ok {
		return
	}
	switch rhs := alias.Rhs().(type) {
	case *types.Named:
		r.record(pkg, surface, obj, rhs.Obj(), spec.Type)
	case *types.Alias:
		r.record(pkg, surface, obj, rhs.Obj(), spec.Type)
	}
}

// addConstReexports records each constant whose value is another package's
// constant of the same type. A constant of another type is a conversion, which
// has a value of its own to answer for.
func (r reexports) addConstReexports(pkg *packages.Package, surface *apiSurface, spec *ast.ValueSpec) {
	if len(spec.Values) != len(spec.Names) {
		return
	}
	for i, name := range spec.Names {
		obj, ok := pkg.TypesInfo.Defs[name].(*types.Const)
		if !ok || !obj.Exported() {
			continue
		}
		sel, ok := spec.Values[i].(*ast.SelectorExpr)
		if !ok {
			continue
		}
		original, ok := pkg.TypesInfo.Uses[sel.Sel].(*types.Const)
		if !ok || !types.Identical(obj.Type(), original.Type()) {
			continue
		}
		r.record(pkg, surface, obj, original, sel)
	}
}

// addForwarder records a function that only forwards to another package's
// function, which Go offers no alias for. It is a copy when it adds nothing
// a call to the original would not do: its body is that one call, returned or,
// without results, made; the call passes the function's own type parameters
// and parameters, each once and in order, spreading a variadic one; and its
// signature is identical to the original's, constraints included. The name
// it gives is free, as an alias's is.
func (r reexports) addForwarder(pkg *packages.Package, surface *apiSurface, decl *ast.FuncDecl) {
	fn, ok := pkg.TypesInfo.Defs[decl.Name].(*types.Func)
	if !ok || !fn.Exported() || decl.Recv != nil || decl.Body == nil || len(decl.Body.List) != 1 {
		return
	}
	sig := fn.Signature()
	call := forwardedCall(decl.Body.List[0], sig.Results().Len())
	if call == nil {
		return
	}
	name := calleeName(call.Fun)
	if name == nil {
		return
	}
	original, ok := pkg.TypesInfo.Uses[name].(*types.Func)
	if !ok || original.Signature().Recv() != nil || !types.Identical(fn.Type(), original.Type()) {
		return
	}
	if !forwardsTypeParams(pkg.TypesInfo.Instances[name], sig) || !forwardsParams(pkg.TypesInfo, call, sig) {
		return
	}
	r.record(pkg, surface, fn, original, call.Fun)
}

// forwardedCall returns the call a forwarding body's one statement makes:
// returned as the only result when the function has results, made as a bare
// statement when it has none.
func forwardedCall(stmt ast.Stmt, results int) *ast.CallExpr {
	var expr ast.Expr
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		if results == 0 || len(stmt.Results) != 1 {
			return nil
		}
		expr = stmt.Results[0]
	case *ast.ExprStmt:
		if results != 0 {
			return nil
		}
		expr = stmt.X
	}
	call, _ := ast.Unparen(expr).(*ast.CallExpr)
	return call
}

// calleeName returns the identifier a call names its function by — p.F, or F
// through a dot import — past any explicit instantiation.
func calleeName(fun ast.Expr) *ast.Ident {
	switch fun := ast.Unparen(fun).(type) {
	case *ast.IndexExpr:
		return calleeName(fun.X)
	case *ast.IndexListExpr:
		return calleeName(fun.X)
	case *ast.SelectorExpr:
		if _, ok := fun.X.(*ast.Ident); ok {
			return fun.Sel
		}
	case *ast.Ident:
		return fun
	}
	return nil
}

// forwardsTypeParams reports whether a call instantiates its callee
// with exactly the forwarding function's own type parameters, in order,
// explicitly or by inference.
func forwardsTypeParams(instance types.Instance, sig *types.Signature) bool {
	params := sig.TypeParams()
	if params.Len() == 0 {
		return instance.TypeArgs.Len() == 0
	}
	if instance.TypeArgs.Len() != params.Len() {
		return false
	}
	for i := range params.Len() {
		if instance.TypeArgs.At(i) != params.At(i) {
			return false
		}
	}
	return true
}

// forwardsParams reports whether a call passes exactly the forwarding
// function's own parameters, each once and in order, spreading the last one
// exactly when it is variadic.
func forwardsParams(info *types.Info, call *ast.CallExpr, sig *types.Signature) bool {
	params := sig.Params()
	if len(call.Args) != params.Len() || call.Ellipsis.IsValid() != sig.Variadic() {
		return false
	}
	for i, arg := range call.Args {
		ident, ok := ast.Unparen(arg).(*ast.Ident)
		if !ok || info.Uses[ident] != params.At(i) {
			return false
		}
	}
	return true
}

// record records copied as a second name for original when original is declared
// on the surface too. A copy of anything else — a package the surface leaves
// out, a dependency — is the surface's only name for it, and is judged
// as the declaration it then is.
func (r reexports) record(pkg *packages.Package, surface *apiSurface, copied, original types.Object, rhs ast.Expr) {
	from := original.Pkg()
	if from == nil || from.Path() == copied.Pkg().Path() || !surface.packages[from.Path()] {
		return
	}
	fset := pkg.Fset
	r[declKey(position(fset, copied.Pos()))] = reexport{
		original: declKey(position(fset, original.Pos())),
		rhs:      toSpan(fset, rhs.Pos(), rhs.End()),
	}
}

// renamed follows a chain of re-exports to the declaration it ends at. Each
// link names a package its copy imports, so the chain ends where the import
// graph does.
func (r reexports) renamed(key string) string {
	for {
		copied, ok := r[key]
		if !ok {
			return key
		}
		key = copied.original
	}
}

// copies reports whether the declaration at key is a copy, which draws
// no verdict of any kind: it adds a name, not a declaration.
func (r reexports) copies(key string) bool {
	_, ok := r[key]
	return ok
}
