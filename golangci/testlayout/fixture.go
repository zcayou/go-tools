package testlayout

import (
	"go/ast"
	"go/token"
	"strings"
)

// fixture is one package-level declaration of a test file that specs may use
// but that registers nothing itself: a function, a type, a variable,
// or a constant, each name of a grouped declaration on its own.
type fixture struct {
	name string

	// ident is where the fixture is declared, in the file set of whichever
	// syntax it was read from; only a fixture the pass holds is reported at it.
	ident *ast.Ident

	// uses are the fixtures this one refers to, through its own declaration
	// or, for a type, through any method declared on it.
	uses []*fixture

	// consumers are the files whose roots reach this fixture.
	consumers map[string]bool
}

// fixtures is every fixture of one directory's test files, across both test
// packages and every build, with the files that consume each one.
type fixtures struct {
	// byFile lists each file's fixtures in declaration order.
	byFile map[string][]*fixture

	// byPackage indexes fixtures by the package a file declares and then
	// by name. A name has more than one fixture only where files of different
	// builds each declare it.
	byPackage map[string]map[string][]*fixture

	// types holds the type each package-level type declaration gives a name,
	// by package, for reading the shape of a composite literal.
	types map[string]map[string]ast.Expr

	// roots lists, per file, the fixtures its roots refer to: everything
	// at the top level of the file that is not a fixture, such as a spec,
	// an entry point, an init function, or a suite hook.
	roots map[string][]*fixture
}

// readFixtures builds the fixtures of files and the files consuming each.
// underTest is the import path of the package under test, through which
// a black-box file refers to a white-box one. sources are files that declare
// no fixtures but may declare a type a composite literal is read through.
func readFixtures(files []testFile, sources []*ast.File, underTest string) fixtures {
	graph := fixtures{
		byFile:    make(map[string][]*fixture),
		byPackage: make(map[string]map[string][]*fixture),
		types:     make(map[string]map[string]ast.Expr),
		roots:     make(map[string][]*fixture),
	}
	for _, source := range sources {
		graph.declareTypes(source)
	}
	for _, file := range files {
		graph.declare(file)
		graph.declareTypes(file.syntax)
	}
	for _, file := range files {
		graph.link(file, underTest)
	}
	for name, refs := range graph.roots {
		graph.consume(name, refs)
	}
	return graph
}

// declare records the fixtures one file declares.
func (g *fixtures) declare(file testFile) {
	pkg := file.syntax.Name.Name
	if g.byPackage[pkg] == nil {
		g.byPackage[pkg] = make(map[string][]*fixture)
	}
	add := func(ident *ast.Ident) {
		f := &fixture{name: ident.Name, ident: ident, consumers: make(map[string]bool)}
		g.byFile[file.name] = append(g.byFile[file.name], f)
		g.byPackage[pkg][ident.Name] = append(g.byPackage[pkg][ident.Name], f)
	}

	for _, decl := range file.syntax.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Recv == nil && isFixtureFunc(decl.Name.Name) {
				add(decl.Name)
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if spec.Name.Name != "_" {
						add(spec.Name)
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.Name != "_" {
							add(name)
						}
					}
				}
			}
		}
	}
}

// isFixtureFunc reports whether a package-level function is a fixture rather
// than a root: init and the go test entry points run whether or not anything
// refers to them.
func isFixtureFunc(name string) bool {
	return name != "_" && name != "init" && !isTestFuncName(name)
}

func (g *fixtures) declareTypes(file *ast.File) {
	pkg := file.Name.Name
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			spec := spec.(*ast.TypeSpec)
			if g.types[pkg] == nil {
				g.types[pkg] = make(map[string]ast.Expr)
			}
			g.types[pkg][spec.Name.Name] = spec.Type
		}
	}
}

// link records what each fixture of one file refers to, and what the file's
// roots refer to.
func (g *fixtures) link(file testFile, underTest string) {
	pkg := file.syntax.Name.Name
	whitebox, _ := strings.CutSuffix(pkg, "_test")
	l := &linker{graph: g, file: file.name, pkg: pkg, whitebox: whitebox}
	l.qualifiers, l.dot = underTestImports(file.syntax, underTest, whitebox)
	l.visit = l.refer
	l.typeOf = l.typeNamed

	own := make(map[*ast.Ident]*fixture)
	for _, f := range g.byFile[file.name] {
		own[f.ident] = f
	}

	for _, decl := range file.syntax.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			// A method goes with its receiver's type, wherever either is declared. One
			// on a type no test file declares belongs to the package under test,
			// and is a root.
			if decl.Recv != nil {
				l.owners = g.byPackage[pkg][receiverName(decl.Recv)]
			} else {
				l.owners = owning(own[decl.Name])
			}
			l.funcDecl(decl)
		case *ast.GenDecl:
			l.genDecl(decl, own)
		}
	}
}

// linker attributes the references one file makes to whatever makes them.
type linker struct {
	resolver

	graph    *fixtures
	file     string
	pkg      string
	whitebox string
	dot      bool

	// owners are the fixtures the declaration being walked belongs to, or none
	// for a root.
	owners []*fixture
}

func (l *linker) refer(name string, qualified bool) {
	var found []*fixture
	switch {
	case qualified:
		found = l.graph.byPackage[l.whitebox][name]
	default:
		found = l.graph.byPackage[l.pkg][name]
		if found == nil && l.dot {
			found = l.graph.byPackage[l.whitebox][name]
		}
	}
	if len(l.owners) == 0 {
		l.graph.roots[l.file] = append(l.graph.roots[l.file], found...)
		return
	}
	for _, owner := range l.owners {
		owner.uses = append(owner.uses, found...)
	}
}

func (l *linker) typeNamed(name string, qualified bool) ast.Expr {
	if qualified {
		return l.graph.types[l.whitebox][name]
	}
	return l.graph.types[l.pkg][name]
}

// genDecl attributes what one top-level declaration refers to. Each name
// of a value specification owns its type and the value it is given; a constant
// repeating the specification before it, as in an iota group, owns what
// that specification refers to.
func (l *linker) genDecl(decl *ast.GenDecl, own map[*ast.Ident]*fixture) {
	var repeated *ast.ValueSpec
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			l.owners = owning(own[spec.Name])
			l.typeSpec(spec)
		case *ast.ValueSpec:
			source := spec
			if decl.Tok == token.CONST && spec.Type == nil && len(spec.Values) == 0 && repeated != nil {
				source = repeated
			} else {
				repeated = spec
			}
			for i, name := range spec.Names {
				values := source.Values
				if len(values) == len(source.Names) {
					values = values[i : i+1]
				}
				l.owners = owning(own[name])
				l.expr(source.Type)
				l.exprs(values)
			}
		}
	}
}

func owning(f *fixture) []*fixture {
	if f == nil {
		return nil
	}
	return []*fixture{f}
}

// receiverName is the name of the type a method's receiver is declared on.
func receiverName(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	for {
		switch t := expr.(type) {
		case *ast.StarExpr:
			expr = t.X
		case *ast.ParenExpr:
			expr = t.X
		case *ast.IndexExpr:
			expr = t.X
		case *ast.IndexListExpr:
			expr = t.X
		case *ast.Ident:
			return t.Name
		default:
			return ""
		}
	}
}

// underTestImports reads how a file refers to the package under test: the names
// it imports it under, declared or renamed, and whether it dot-imports it.
// whitebox is the name the package declares, which is what an import that does
// not rename it is known by.
func underTestImports(file *ast.File, underTest, whitebox string) (qualifiers []string, dot bool) {
	for _, spec := range file.Imports {
		if importPath(spec) != underTest {
			continue
		}
		name := whitebox
		if spec.Name != nil {
			name = spec.Name.Name
		}
		switch name {
		case ".":
			dot = true
		case "_":
		default:
			qualifiers = append(qualifiers, name)
		}
	}
	return qualifiers, dot
}

// consume marks every fixture reachable from refs as consumed by file.
func (g *fixtures) consume(file string, refs []*fixture) {
	queue := refs
	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]
		if f.consumers[file] {
			continue
		}
		f.consumers[file] = true
		queue = append(queue, f.uses...)
	}
}
