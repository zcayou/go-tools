// Package enum holds the one definition of an enum that every linter in this
// module shares, along with its members and the tokens they render as.
//
// An enum is a defined, non-generic type whose own package declares typed
// constants of it in non-test files. Those constants are its members,
// and membership is judged by value, so a constant re-exported under another
// name is still a member. A constant declared in a test file makes nothing
// an enum: the members are what production publishes.
//
// How the constants are declared can say the type is no closed set at all.
// Each rule reads the declaration alone, never how the constants are used
// elsewhere:
//
//   - A flag set is no enum. Its values combine, so a value no constant
//     names is ordinary. A type is one when any of its constants is declared
//     by a << shift, written or repeated from the line above, as in 1 << iota.
//   - A quantity is no enum. Its values are amounts, and its constants
//     are units. A type is one when any of its constants is declared
//     by multiplying another of them, as in MB = 1024 * KB. Adding to one,
//     as in B = A + 1, is how an enumeration counts, and keeps the type
//     an enum.
//   - A count sentinel is no member. It marks where an enumeration ends rather
//     than naming a value of it, so it needs no name, and no value is wrong
//     for holding it. A constant is one when it is unexported, it closes
//     an iota run — it is the last line of its const block and repeats
//     the run's iota expression — and its own package uses it as an array
//     length or as one side of an ordering comparison. Its type stays
//     an enum.
package enum

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"
)

// Enum is one enum.
type Enum struct {
	// Type is the defined type.
	Type *types.Named

	// Members are the constants its package declares of it, in declaration
	// order, count sentinels left out.
	Members []*types.Const

	// pkg is the package declaring it, whose syntax its String is read from.
	pkg Package
}

// Package is what the definition reads of the package declaring an enum:
// its syntax and type information. A packages.Package loaded with both holds
// them, and so does an analysis.Pass.
type Package struct {
	Fset  *token.FileSet
	Files []*ast.File
	Types *types.Package
	Info  *types.Info
}

// Declared returns the enums pkg declares, in the order of their type names.
// How each constant is declared decides whether its type is an enum and whether
// it is a member, and whether it sits in a test file decides whether it counts.
func Declared(pkg Package) []*Enum {
	declared := newDeclarations(pkg)
	byType := map[*types.Named]*Enum{}
	var found []*Enum
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		member, ok := scope.Lookup(name).(*types.Const)
		if !ok || declared.inTest(member.Pos()) {
			continue
		}
		named, ok := types.Unalias(member.Type()).(*types.Named)
		if !ok || named.Obj().Pkg() != pkg.Types || named.TypeParams().Len() > 0 {
			continue
		}
		e := byType[named]
		if e == nil {
			e = &Enum{Type: named, pkg: pkg}
			byType[named] = e
			found = append(found, e)
		}
		e.Members = append(e.Members, member)
	}

	found = slices.DeleteFunc(found, func(e *Enum) bool {
		if slices.ContainsFunc(e.Members, declared.combines) {
			return true
		}
		e.Members = slices.DeleteFunc(e.Members, declared.sentinel)
		return len(e.Members) == 0
	})
	for _, e := range found {
		slices.SortFunc(e.Members, func(a, b *types.Const) int { return int(a.Pos() - b.Pos()) })
	}
	slices.SortFunc(found, func(a, b *Enum) int { return strings.Compare(a.Type.Obj().Name(), b.Type.Obj().Name()) })
	return found
}

// Ordinal reports whether the enum's underlying type is an integer.
func (v *Enum) Ordinal() bool {
	basic, ok := v.Type.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsInteger != 0
}

// Textual reports whether the enum's underlying type is a string.
func (v *Enum) Textual() bool {
	basic, ok := v.Type.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

// Values returns the distinct values of the enum's members, in the order
// their first member is declared.
func (v *Enum) Values() []constant.Value {
	var values []constant.Value
	for _, member := range v.Members {
		if !slices.ContainsFunc(values, func(seen constant.Value) bool { return constant.Compare(seen, token.EQL, member.Val()) }) {
			values = append(values, member.Val())
		}
	}
	return values
}

// Member reports whether value is some member's.
func (v *Enum) Member(value constant.Value) bool {
	return slices.ContainsFunc(v.Members, func(member *types.Const) bool {
		return sameKind(member.Val(), value) && constant.Compare(member.Val(), token.EQL, value)
	})
}

// sameKind reports whether two constants can be compared: both numeric,
// or both of one non-numeric kind.
func sameKind(a, b constant.Value) bool {
	numeric := func(v constant.Value) bool {
		switch v.Kind() {
		case constant.Int, constant.Float, constant.Complex:
			return true
		default:
			return false
		}
	}
	return a.Kind() == b.Kind() || numeric(a) && numeric(b)
}

// Name returns a type's String() string method on a value receiver — what
// it publishes as each value's name — or nil when it declares none.
func Name(named *types.Named) *types.Func {
	for method := range named.Methods() {
		if method.Name() != "String" {
			continue
		}
		sig := method.Signature()
		if _, pointer := sig.Recv().Type().(*types.Pointer); pointer {
			return nil
		}
		if sig.Params().Len() != 0 || sig.Results().Len() != 1 ||
			!types.Identical(sig.Results().At(0).Type(), types.Typ[types.String]) {
			return nil
		}
		return method
	}
	return nil
}

// declarations is what a package's non-test syntax says of its package-level
// constants: the expression each takes its value from, which of them close
// an iota run, and which the package uses as a bound.
type declarations struct {
	pkg Package

	// values holds the expression each constant's value comes from: its own,
	// or the one it repeats from the line above.
	values map[*types.Const]ast.Expr

	// closing holds the constants that close an iota run.
	closing map[*types.Const]bool

	// bounds holds the constants used as an array length or as one side
	// of an ordering comparison.
	bounds map[*types.Const]bool
}

func newDeclarations(pkg Package) *declarations {
	d := &declarations{
		pkg:     pkg,
		values:  map[*types.Const]ast.Expr{},
		closing: map[*types.Const]bool{},
		bounds:  map[*types.Const]bool{},
	}
	if pkg.Info == nil {
		return d
	}
	for _, file := range pkg.Files {
		if d.inTest(file.Pos()) {
			continue
		}
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.CONST {
				d.constBlock(gen)
			}
		}
		ast.Inspect(file, d.bound)
	}
	return d
}

func (d *declarations) inTest(pos token.Pos) bool {
	return strings.HasSuffix(d.pkg.Fset.PositionFor(pos, false).Filename, "_test.go")
}

// constBlock records each constant's value expression, repeating the last one
// written for a line that writes none, as the language does.
func (d *declarations) constBlock(gen *ast.GenDecl) {
	var last []ast.Expr
	for i, spec := range gen.Specs {
		spec := spec.(*ast.ValueSpec)
		values := spec.Values
		repeated := len(values) == 0
		if repeated {
			values = last
		} else {
			last = values
		}
		for j, name := range spec.Names {
			member, ok := d.pkg.Info.Defs[name].(*types.Const)
			if !ok || j >= len(values) {
				continue
			}
			d.values[member] = values[j]
			if repeated && i == len(gen.Specs)-1 && d.mentionsIota(values[j]) {
				d.closing[member] = true
			}
		}
	}
}

func (d *declarations) mentionsIota(expr ast.Expr) bool {
	counter := types.Universe.Lookup("iota")
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && d.pkg.Info.Uses[id] == counter {
			found = true
		}
		return !found
	})
	return found
}

// bound records the constants a node uses as an array length or as one side
// of an ordering comparison.
func (d *declarations) bound(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.ArrayType:
		if n.Len != nil {
			d.markBound(n.Len)
		}
	case *ast.BinaryExpr:
		switch n.Op {
		case token.LSS, token.LEQ, token.GTR, token.GEQ:
			d.markBound(n.X)
			d.markBound(n.Y)
		}
	}
	return true
}

func (d *declarations) markBound(expr ast.Expr) {
	if named := d.constantOf(expr); named != nil {
		d.bounds[named] = true
	}
}

// constantOf returns the package-level constant expr names, seen through
// parentheses and conversions, or nil.
func (d *declarations) constantOf(expr ast.Expr) *types.Const {
	var id *ast.Ident
	switch expr := d.unconvert(expr).(type) {
	case *ast.Ident:
		id = expr
	case *ast.SelectorExpr:
		id = expr.Sel
	default:
		return nil
	}
	named, _ := d.pkg.Info.Uses[id].(*types.Const)
	return named
}

// unconvert strips parentheses and conversions from expr.
func (d *declarations) unconvert(expr ast.Expr) ast.Expr {
	for {
		expr = ast.Unparen(expr)
		call, ok := expr.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return expr
		}
		if tv, ok := d.pkg.Info.Types[call.Fun]; !ok || !tv.IsType() {
			return expr
		}
		expr = call.Args[0]
	}
}

// combines reports whether a constant is declared the way a flag set's
// or a quantity's is: by a shift, or by multiplying another constant of its
// type.
func (d *declarations) combines(member *types.Const) bool {
	value, ok := d.values[member]
	if !ok {
		return false
	}
	binary, ok := d.unconvert(value).(*ast.BinaryExpr)
	if !ok {
		return false
	}
	switch binary.Op {
	case token.SHL:
		return true
	case token.MUL:
		for _, operand := range []ast.Expr{binary.X, binary.Y} {
			if other := d.constantOf(operand); other != nil && other != member &&
				types.Identical(other.Type(), member.Type()) {
				return true
			}
		}
	}
	return false
}

// sentinel reports whether a constant is a count sentinel rather than
// a member.
func (d *declarations) sentinel(member *types.Const) bool {
	return !member.Exported() && d.closing[member] && d.bounds[member]
}
