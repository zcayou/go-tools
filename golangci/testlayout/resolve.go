package testlayout

import (
	"go/ast"
	"go/token"
	"slices"
)

// resolver walks one file's syntax the way Go scopes it and hands every
// identifier that does not name something local to visit, which decides
// whether a package-level declaration answers to it.
//
// Resolution is syntactic, because the linter runs without type information
// and reads half of what it judges from disk. Go makes that sound at package
// scope: a package-level name is declared once, and one clashing
// with a dot-imported name does not compile. What remains is shadowing, which
// the scopes here follow, and the key of a composite literal, which names
// a field or a value depending on a type the syntax may not show; such a key
// is read as a field name unless the literal's type is visibly a map, slice,
// or array.
type resolver struct {
	scopes []map[string]bool

	// qualifiers are the names the file imports the package under test under,
	// which make a selector on one of them a reference into that package.
	qualifiers []string

	// visit receives a name the walk could not resolve locally. underTest
	// reports that it was qualified by one of qualifiers.
	visit func(name string, underTest bool)

	// typeOf returns the type a package-level type declaration gives a name,
	// or nil, for reading the shape of a composite literal.
	typeOf func(name string, underTest bool) ast.Expr
}

func (r *resolver) push() {
	r.scopes = append(r.scopes, make(map[string]bool))
}

func (r *resolver) pop() {
	r.scopes = r.scopes[:len(r.scopes)-1]
}

func (r *resolver) declare(id *ast.Ident) {
	if id != nil && id.Name != "_" {
		r.scopes[len(r.scopes)-1][id.Name] = true
	}
}

func (r *resolver) local(name string) bool {
	return slices.ContainsFunc(r.scopes, func(scope map[string]bool) bool { return scope[name] })
}

func (r *resolver) ident(id *ast.Ident) {
	if !r.local(id.Name) {
		r.visit(id.Name, false)
	}
}

// funcDecl walks a package-level function or method. Type parameters,
// the receiver's among them, are in scope for the signature; parameter names
// are in scope for the body only.
func (r *resolver) funcDecl(decl *ast.FuncDecl) {
	r.push()
	defer r.pop()

	if decl.Recv != nil {
		for _, field := range decl.Recv.List {
			r.receiverType(field.Type)
		}
	}
	r.typeParams(decl.Type.TypeParams)
	r.signature(decl.Type)
	if decl.Recv != nil {
		r.declareFields(decl.Recv)
	}
	r.declareFields(decl.Type.Params)
	r.declareFields(decl.Type.Results)
	if decl.Body != nil {
		r.stmts(decl.Body.List)
	}
}

// receiverType walks a receiver's type, in which the indices of a generic
// receiver declare its type parameters rather than referring to anything.
func (r *resolver) receiverType(expr ast.Expr) {
	for {
		switch t := expr.(type) {
		case *ast.StarExpr:
			expr = t.X
			continue
		case *ast.ParenExpr:
			expr = t.X
			continue
		case *ast.IndexExpr:
			r.declareIdent(t.Index)
			r.expr(t.X)
		case *ast.IndexListExpr:
			for _, index := range t.Indices {
				r.declareIdent(index)
			}
			r.expr(t.X)
		default:
			r.expr(expr)
		}
		return
	}
}

func (r *resolver) declareIdent(expr ast.Expr) {
	if id, ok := expr.(*ast.Ident); ok {
		r.declare(id)
	}
}

// typeSpec walks a type declaration, whose type parameters are in scope
// for one another's constraints and for the type.
func (r *resolver) typeSpec(spec *ast.TypeSpec) {
	r.push()
	defer r.pop()

	r.typeParams(spec.TypeParams)
	r.expr(spec.Type)
}

func (r *resolver) typeParams(list *ast.FieldList) {
	r.declareFields(list)
	r.fieldTypes(list)
}

// signature walks the parameter and result types of a function, leaving
// the names to the caller, which declares them where the body can see them.
func (r *resolver) signature(fn *ast.FuncType) {
	r.fieldTypes(fn.Params)
	r.fieldTypes(fn.Results)
}

func (r *resolver) declareFields(list *ast.FieldList) {
	if list == nil {
		return
	}
	for _, field := range list.List {
		for _, name := range field.Names {
			r.declare(name)
		}
	}
}

// fieldTypes walks the types of a field list. The names in it declare
// parameters or struct fields, which nothing outside a selector refers to.
func (r *resolver) fieldTypes(list *ast.FieldList) {
	if list == nil {
		return
	}
	for _, field := range list.List {
		r.expr(field.Type)
	}
}

func (r *resolver) exprs(list []ast.Expr) {
	for _, expr := range list {
		r.expr(expr)
	}
}

func (r *resolver) expr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.Ident:
		r.ident(e)
	case *ast.CompositeLit:
		r.compositeLit(e, nil)
	case *ast.FuncLit:
		r.push()
		r.signature(e.Type)
		r.declareFields(e.Type.Params)
		r.declareFields(e.Type.Results)
		r.stmts(e.Body.List)
		r.pop()
	case *ast.ParenExpr:
		r.expr(e.X)
	case *ast.SelectorExpr:
		// The selected name is a field, a method, or a member of the package
		// the operand names; only the last can be a declaration judged here.
		if pkg, ok := e.X.(*ast.Ident); ok && !r.local(pkg.Name) && slices.Contains(r.qualifiers, pkg.Name) {
			r.visit(e.Sel.Name, true)
			return
		}
		r.expr(e.X)
	case *ast.IndexExpr:
		r.expr(e.X)
		r.expr(e.Index)
	case *ast.IndexListExpr:
		r.expr(e.X)
		r.exprs(e.Indices)
	case *ast.SliceExpr:
		r.exprs([]ast.Expr{e.X, e.Low, e.High, e.Max})
	case *ast.TypeAssertExpr:
		r.expr(e.X)
		r.expr(e.Type)
	case *ast.CallExpr:
		r.expr(e.Fun)
		r.exprs(e.Args)
	case *ast.StarExpr:
		r.expr(e.X)
	case *ast.UnaryExpr:
		r.expr(e.X)
	case *ast.BinaryExpr:
		r.expr(e.X)
		r.expr(e.Y)
	default:
		r.typeExpr(expr)
	}
}

// typeExpr walks a type written out. Struct fields, parameters, and interface
// methods name nothing the walk resolves, so only their types are walked.
func (r *resolver) typeExpr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.ArrayType:
		r.expr(e.Len)
		r.expr(e.Elt)
	case *ast.StructType:
		r.fieldTypes(e.Fields)
	case *ast.FuncType:
		r.signature(e)
	case *ast.InterfaceType:
		r.fieldTypes(e.Methods)
	case *ast.MapType:
		r.expr(e.Key)
		r.expr(e.Value)
	case *ast.ChanType:
		r.expr(e.Value)
	case *ast.Ellipsis:
		r.expr(e.Elt)
	}
}

// compositeLit walks a composite literal. implied is the type an enclosing
// literal gives one whose own type is elided.
func (r *resolver) compositeLit(lit *ast.CompositeLit, implied ast.Expr) {
	typ := lit.Type
	if typ != nil {
		r.expr(typ)
	} else {
		typ = implied
	}

	keyed, key, elem := r.literalShape(typ)
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			r.element(elt, elem)
			continue
		}
		if id, ok := kv.Key.(*ast.Ident); ok {
			if keyed {
				r.ident(id)
			}
		} else {
			r.element(kv.Key, key)
		}
		r.element(kv.Value, elem)
	}
}

func (r *resolver) element(expr, implied ast.Expr) {
	if lit, ok := expr.(*ast.CompositeLit); ok {
		r.compositeLit(lit, implied)
		return
	}
	r.expr(expr)
}

// literalShape reads a composite literal's type far enough to say whether its
// keys are values, as a map's, a slice's, and an array's are, rather than
// field names, and what type it implies for an elided key or element. A type
// the syntax does not show, an imported one or a type parameter, reads
// as a struct.
func (r *resolver) literalShape(typ ast.Expr) (keyed bool, key, elem ast.Expr) {
	seen := make(map[ast.Expr]bool)
	for typ != nil && !seen[typ] {
		seen[typ] = true
		switch t := typ.(type) {
		case *ast.ArrayType:
			return true, nil, t.Elt
		case *ast.MapType:
			return true, t.Key, t.Value
		case *ast.ParenExpr:
			typ = t.X
		case *ast.StarExpr:
			typ = t.X
		case *ast.IndexExpr:
			typ = t.X
		case *ast.IndexListExpr:
			typ = t.X
		case *ast.Ident:
			if r.local(t.Name) {
				return false, nil, nil
			}
			typ = r.typeOf(t.Name, false)
		case *ast.SelectorExpr:
			pkg, ok := t.X.(*ast.Ident)
			if !ok || r.local(pkg.Name) || !slices.Contains(r.qualifiers, pkg.Name) {
				return false, nil, nil
			}
			typ = r.typeOf(t.Sel.Name, true)
		default:
			return false, nil, nil
		}
	}
	return false, nil, nil
}

func (r *resolver) stmts(list []ast.Stmt) {
	for _, stmt := range list {
		r.stmt(stmt)
	}
}

func (r *resolver) stmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		r.push()
		r.stmts(s.List)
		r.pop()
	case *ast.DeclStmt:
		if gen, ok := s.Decl.(*ast.GenDecl); ok {
			r.localDecl(gen)
		}
	case *ast.AssignStmt:
		r.exprs(s.Rhs)
		if s.Tok != token.DEFINE {
			r.exprs(s.Lhs)
			return
		}
		for _, lhs := range s.Lhs {
			r.declareIdent(lhs)
		}
	case *ast.ExprStmt:
		r.expr(s.X)
	case *ast.SendStmt:
		r.expr(s.Chan)
		r.expr(s.Value)
	case *ast.IncDecStmt:
		r.expr(s.X)
	case *ast.GoStmt:
		r.expr(s.Call)
	case *ast.DeferStmt:
		r.expr(s.Call)
	case *ast.ReturnStmt:
		r.exprs(s.Results)
	case *ast.LabeledStmt:
		r.stmt(s.Stmt)
	default:
		r.controlStmt(stmt)
	}
}

// controlStmt walks a statement that opens scopes of its own: its header's,
// and one per clause.
func (r *resolver) controlStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		r.push()
		r.stmt(s.Init)
		r.expr(s.Cond)
		r.stmt(s.Body)
		r.stmt(s.Else)
		r.pop()
	case *ast.SwitchStmt:
		r.push()
		r.stmt(s.Init)
		r.expr(s.Tag)
		for _, clause := range s.Body.List {
			if clause, ok := clause.(*ast.CaseClause); ok {
				r.exprs(clause.List)
				r.clause(nil, clause.Body)
			}
		}
		r.pop()
	case *ast.TypeSwitchStmt:
		r.typeSwitch(s)
	case *ast.SelectStmt:
		for _, clause := range s.Body.List {
			if clause, ok := clause.(*ast.CommClause); ok {
				r.push()
				r.stmt(clause.Comm)
				r.stmts(clause.Body)
				r.pop()
			}
		}
	case *ast.ForStmt:
		r.push()
		r.stmt(s.Init)
		r.expr(s.Cond)
		r.stmt(s.Post)
		r.stmt(s.Body)
		r.pop()
	case *ast.RangeStmt:
		r.push()
		r.expr(s.X)
		if s.Tok == token.DEFINE {
			r.declareIdent(s.Key)
			r.declareIdent(s.Value)
		} else {
			r.expr(s.Key)
			r.expr(s.Value)
		}
		r.stmt(s.Body)
		r.pop()
	}
}

// typeSwitch walks a type switch, whose binding is declared afresh in each
// clause.
func (r *resolver) typeSwitch(s *ast.TypeSwitchStmt) {
	r.push()
	defer r.pop()

	r.stmt(s.Init)
	var bound *ast.Ident
	switch assign := s.Assign.(type) {
	case *ast.AssignStmt:
		if len(assign.Lhs) == 1 {
			bound, _ = assign.Lhs[0].(*ast.Ident)
		}
		r.exprs(assign.Rhs)
	case *ast.ExprStmt:
		r.expr(assign.X)
	}
	for _, clause := range s.Body.List {
		if clause, ok := clause.(*ast.CaseClause); ok {
			r.exprs(clause.List)
			r.clause(bound, clause.Body)
		}
	}
}

// clause walks the body of a case, a scope of its own, with bound declared
// in it when the case belongs to a type switch that binds a name.
func (r *resolver) clause(bound *ast.Ident, body []ast.Stmt) {
	r.push()
	defer r.pop()

	r.declare(bound)
	r.stmts(body)
}

// localDecl walks a declaration inside a function. A variable or constant
// is in scope after its specification, so its initializer sees what it shadows;
// a type is in scope from its own name on.
func (r *resolver) localDecl(gen *ast.GenDecl) {
	for _, spec := range gen.Specs {
		switch spec := spec.(type) {
		case *ast.ValueSpec:
			r.expr(spec.Type)
			r.exprs(spec.Values)
			for _, name := range spec.Names {
				r.declare(name)
			}
		case *ast.TypeSpec:
			r.declare(spec.Name)
			r.typeSpec(spec)
		}
	}
}
