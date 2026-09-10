package engine

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

type identScan struct {
	declared   map[string]declaration
	uses       map[string][]token.Position
	selfSpans  map[string][]span
	blankSpans spanIndex
	reexports  map[string]reexport
}

// reexport is one copy on the declared surface: a declaration renaming one
// another surface package makes. The scan holds it under the copy's own
// declaration key.
type reexport struct {
	// original is the declaration key of what the copy renames, itself possibly
	// a copy.
	original string
	// rhs is the copy's right-hand side, where its one reference to the original
	// defines the second name rather than using the first.
	rhs span
}

// unusedExportedIdents reports exported package-level types, constants,
// variables, and functions with no reference the view admits anywhere
// in the loaded packages. Functions are covered here rather than
// by reachability alone because a function that is a call-graph root
// is reachable by construction; only a reference scan can tell that nothing
// calls it.
func unusedExportedIdents(
	pkgs []*packages.Package,
	facts map[*packages.Package]fileFacts,
	surface *apiSurface,
	v view,
) []declaration {
	scan := &identScan{
		declared:   map[string]declaration{},
		uses:       map[string][]token.Position{},
		selfSpans:  map[string][]span{},
		blankSpans: spanIndex{},
		reexports:  map[string]reexport{},
	}
	for _, pkg := range pkgs {
		scan.addPackage(pkg, facts[pkg], surface, v)
	}
	scan.forwardReexports()

	var unused []declaration
	for key, decl := range scan.declared {
		if scan.isUsed(key) {
			continue
		}
		unused = append(unused, decl)
	}
	sortDeclarations(unused)
	return unused
}

// unreferencedTypes returns the declaration keys of exported types nothing
// reaches and no API surface exempts, the types whose methods die with them.
// An exempt type is not among them: it stands in for the consumers this run
// cannot see, so its methods keep both their exemption and their roots.
func unreferencedTypes(idents []declaration, surface *apiSurface) map[string]bool {
	dead := map[string]bool{}
	for _, decl := range idents {
		if decl.kind == KindType && !surface.shields(decl) {
			dead[declKey(decl.pos)] = true
		}
	}
	return dead
}

// addPackage records the package's declarations, re-exports, uses, and spans.
// A synthesized root program contributes uses and nothing else: its references
// are what make a declared entry point count, while its own declarations
// are never candidates.
func (s *identScan) addPackage(pkg *packages.Package, facts fileFacts, surface *apiSurface, v view) {
	if !synthesizedPackage(pkg.PkgPath) {
		s.addDeclarations(pkg, facts)
		s.addReexports(pkg, facts, surface)
	}
	s.addUses(pkg, v)
	s.addSpans(pkg)
}

// addDeclarations records the exported package-level declarations a finding
// could name.
func (s *identScan) addDeclarations(pkg *packages.Package, facts fileFacts) {
	fset := pkg.Fset

	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		var kind Kind
		var verdict Verdict
		switch obj.(type) {
		case *types.TypeName:
			kind, verdict = KindType, VerdictUnusedType
		case *types.Const:
			kind, verdict = KindConst, VerdictUnusedConst
		case *types.Var:
			kind, verdict = KindVar, VerdictUnusedVar
		case *types.Func:
			kind, verdict = KindFunc, VerdictUnusedFunc
		default:
			continue
		}
		pos := position(fset, obj.Pos())
		if testFile(pos.Filename) || facts.generated[pos.Filename] {
			continue
		}
		if kind == KindFunc && facts.linknamed[name] {
			continue
		}
		s.declared[declKey(pos)] = declaration{
			pos: pos, verdict: verdict, name: obj.Name(),
			kind: kind, pkg: pkg.PkgPath, exported: true,
		}
	}

}

// addReexports records the package's re-exports: exported declarations
// on the declared surface whose whole right-hand side names a declaration
// another surface package makes. Only a candidate can be one, so a copy
// in a test or generated file stays what it was.
func (s *identScan) addReexports(pkg *packages.Package, facts fileFacts, surface *apiSurface) {
	if !surface.packages[pkg.PkgPath] {
		return
	}
	for _, file := range pkg.Syntax {
		name := position(pkg.Fset, file.Pos()).Filename
		if testFile(name) || facts.generated[name] {
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					s.addTypeReexport(pkg, surface, spec)
				case *ast.ValueSpec:
					if gen.Tok == token.CONST {
						s.addConstReexports(pkg, surface, spec)
					}
				}
			}
		}
	}
}

// addTypeReexport records an alias of another package's named type, generic
// instantiations included: an alias is the type it names, so everything
// spelled through it spells the original.
func (s *identScan) addTypeReexport(pkg *packages.Package, surface *apiSurface, spec *ast.TypeSpec) {
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
		s.recordReexport(pkg, surface, obj, rhs.Obj(), spec.Type)
	case *types.Alias:
		s.recordReexport(pkg, surface, obj, rhs.Obj(), spec.Type)
	}
}

// addConstReexports records each constant whose value is another package's
// constant of the same type. A constant of another type is a conversion, which
// has a value of its own to answer for.
func (s *identScan) addConstReexports(pkg *packages.Package, surface *apiSurface, spec *ast.ValueSpec) {
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
		s.recordReexport(pkg, surface, obj, original, sel)
	}
}

// recordReexport records copied as a second name for original when original
// is declared on the surface too. A copy of anything else — a package
// the surface leaves out, a dependency — is the surface's only name for it,
// and is judged as the declaration it then is.
func (s *identScan) recordReexport(
	pkg *packages.Package,
	surface *apiSurface,
	copied, original types.Object,
	rhs ast.Expr,
) {
	from := original.Pkg()
	if from == nil || from.Path() == copied.Pkg().Path() || !surface.packages[from.Path()] {
		return
	}
	fset := pkg.Fset
	s.reexports[declKey(position(fset, copied.Pos()))] = reexport{
		original: declKey(position(fset, original.Pos())),
		rhs:      toSpan(fset, rhs.Pos(), rhs.End()),
	}
}

// forwardReexports folds each re-export into the declaration at the end of its
// chain. The copy stops being a candidate, every use of it becomes a use
// of that declaration, and its right-hand side joins the declaration's own
// extent: defining a second name is not a use of the first, and counting
// it as one would let a dead declaration hide behind any copy of it.
func (s *identScan) forwardReexports() {
	for key, copied := range s.reexports {
		original := s.renamed(key)
		delete(s.declared, key)
		s.uses[original] = append(s.uses[original], s.uses[key]...)
		s.selfSpans[original] = append(s.selfSpans[original], copied.rhs)
	}
}

// renamed follows a chain of re-exports to the declaration it ends at. Each
// link names a package its copy imports, so the chain ends where the import
// graph does.
func (s *identScan) renamed(key string) string {
	for {
		copied, ok := s.reexports[key]
		if !ok {
			return key
		}
		key = copied.original
	}
}

// addUses records every reference to an exported package-level declaration
// that the view admits.
func (s *identScan) addUses(pkg *packages.Package, v view) {
	if !v.admitsPackage(pkg.PkgPath) {
		return
	}
	fset := pkg.Fset

	for ident, obj := range pkg.TypesInfo.Uses {
		if !obj.Exported() || obj.Pkg() == nil || obj.Parent() != obj.Pkg().Scope() {
			continue
		}
		pos := position(fset, ident.Pos())
		if !v.admitsFile(pos.Filename) {
			continue
		}
		key := declKey(position(fset, obj.Pos()))
		s.uses[key] = append(s.uses[key], pos)
	}
}

// addSpans records the regions a reference does not count from: a declaration's
// own extent, and the inert blank assertions that name a type without holding
// anything behind it.
func (s *identScan) addSpans(pkg *packages.Package) {
	fset := pkg.Fset

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil || len(decl.Recv.List) == 0 {
					// A function's own body is its self span, so that recursion does not read
					// as a use.
					if obj, ok := pkg.TypesInfo.Defs[decl.Name]; ok {
						key := declKey(position(fset, obj.Pos()))
						s.selfSpans[key] = append(s.selfSpans[key], toSpan(fset, decl.Pos(), decl.End()))
					}
					continue
				}
				ident := receiverBase(decl.Recv.List[0].Type)
				if ident == nil {
					continue
				}
				obj, ok := pkg.TypesInfo.Uses[ident]
				if !ok {
					continue
				}
				key := declKey(position(fset, obj.Pos()))
				s.selfSpans[key] = append(s.selfSpans[key], toSpan(fset, decl.Pos(), decl.End()))
			case *ast.GenDecl:
				switch decl.Tok {
				case token.TYPE:
					for _, spec := range decl.Specs {
						typeSpec := spec.(*ast.TypeSpec)
						obj, ok := pkg.TypesInfo.Defs[typeSpec.Name]
						if !ok {
							continue
						}
						key := declKey(position(fset, obj.Pos()))
						s.selfSpans[key] = append(s.selfSpans[key], toSpan(fset, typeSpec.Pos(), typeSpec.End()))
					}
				case token.VAR:
					for _, spec := range decl.Specs {
						valueSpec := spec.(*ast.ValueSpec)
						if allBlank(valueSpec.Names) {
							s.blankSpans.add(blankAssertionSpans(pkg, valueSpec)...)
						}
					}
				}
			}
		}
	}
}

// isUsed reports whether the declaration has at least one reference outside its
// own declaration and blank assertions.
func (s *identScan) isUsed(key string) bool {
	return usedOutsideSpans(s.uses[key], s.selfSpans[key], s.blankSpans)
}
