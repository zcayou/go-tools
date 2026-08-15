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
}

// unusedExportedIdents reports exported package-level types, constants,
// variables, and functions with no reference the view admits anywhere
// in the loaded packages. Functions are covered here rather than
// by reachability alone because a function that is a call-graph root
// is reachable by construction; only a reference scan can tell that nothing
// calls it.
func unusedExportedIdents(pkgs []*packages.Package, facts map[*packages.Package]fileFacts, v view) []declaration {
	scan := &identScan{
		declared:   map[string]declaration{},
		uses:       map[string][]token.Position{},
		selfSpans:  map[string][]span{},
		blankSpans: spanIndex{},
	}
	for _, pkg := range pkgs {
		scan.addPackage(pkg, facts[pkg], v)
	}

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

// addPackage records the package's declarations, uses, and spans. A synthesized
// root program contributes uses and nothing else: its references are what make
// a declared entry point count, while its own declarations are never
// candidates.
func (s *identScan) addPackage(pkg *packages.Package, facts fileFacts, v view) {
	if !synthesizedPackage(pkg.PkgPath) {
		s.addDeclarations(pkg, facts)
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
