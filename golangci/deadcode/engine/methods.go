package engine

import (
	"go/ast"
	"go/token"
	"go/types"
	"maps"

	"golang.org/x/tools/go/packages"
)

type methodScan struct {
	declared map[string]declaration
	// funcs holds the declared methods themselves, which interface satisfaction
	// is decided against.
	funcs map[string]*types.Func
}

type methodReferenceScan struct {
	interfaceMethods    map[string]declaration
	interfaceMethodUses map[string][]token.Position
	// concreteMethodUses holds syntactic selections of the declaration itself;
	// dispatched holds the uses dynamic dispatch confers on implementations. They
	// are separate because they answer different questions — whether anything
	// names the method, and whether the program can invoke it through an interface
	// — and the verdicts consult each on its own.
	concreteMethodUses    map[string][]token.Position
	dispatched            map[string][]token.Position
	concreteMethodsByName map[string][]concreteMethod
	interfaceMethodSpans  map[string][]span
	concreteMethodSpans   map[string][]span
	blankSpans            spanIndex
	// generated holds the files no verdict is reported in. It filters what
	// the interface-method scan reports, never what it indexes: the index answers
	// whether this analysis can see an interface's call sites, and a generated
	// file's are as visible as any other.
	generated map[string]bool
	evidence  *evidence
}

type concreteMethod struct {
	fn  *types.Func
	key string
}

// newMethodScan collects the exported methods declared by the analyzed
// packages, the candidates both method verdicts are drawn from.
func newMethodScan(pkgs []*packages.Package, facts map[*packages.Package]fileFacts) *methodScan {
	scan := &methodScan{
		declared: map[string]declaration{},
		funcs:    map[string]*types.Func{},
	}
	for _, pkg := range pkgs {
		scan.addPackage(pkg, facts[pkg])
	}
	return scan
}

func (s *methodScan) addPackage(pkg *packages.Package, facts fileFacts) {
	if synthesizedPackage(pkg.PkgPath) {
		return
	}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
				continue
			}
			obj, ok := pkg.TypesInfo.Defs[funcDecl.Name].(*types.Func)
			if !ok || !obj.Exported() {
				continue
			}
			pos := position(pkg.Fset, obj.Pos())
			if testFile(pos.Filename) || facts.generated[pos.Filename] {
				continue
			}
			recv := receiverBase(funcDecl.Recv.List[0].Type)
			if recv == nil {
				continue
			}
			key := declKey(pos)
			s.declared[key] = declaration{
				pos: pos, verdict: VerdictUnusedMethod, name: recv.Name + "." + obj.Name(),
				kind: KindMethod, pkg: pkg.PkgPath, exported: true, owner: receiverTypeKey(pkg, funcDecl),
			}
			s.funcs[key] = obj
		}
	}
}

func newMethodReferenceScan(
	pkgs []*packages.Package,
	facts map[*packages.Package]fileFacts,
	ev *evidence,
	v view,
) *methodReferenceScan {
	scan := &methodReferenceScan{
		interfaceMethods:      map[string]declaration{},
		interfaceMethodUses:   map[string][]token.Position{},
		concreteMethodUses:    map[string][]token.Position{},
		dispatched:            map[string][]token.Position{},
		concreteMethodsByName: map[string][]concreteMethod{},
		interfaceMethodSpans:  map[string][]span{},
		concreteMethodSpans:   map[string][]span{},
		blankSpans:            spanIndex{},
		generated:             map[string]bool{},
		evidence:              ev,
	}
	for _, pkg := range pkgs {
		maps.Copy(scan.generated, facts[pkg].generated)
		scan.addPackageDeclarations(pkg)
	}
	for _, pkg := range pkgs {
		scan.addPackageUses(pkg, v)
	}
	return scan
}

// addPackageDeclarations records the method and interface declarations
// the package holds. A synthesized root program contributes only its blank
// assertion spans — the one declaration-side fact that filters uses rather than
// adding candidates.
func (s *methodReferenceScan) addPackageDeclarations(pkg *packages.Package) {
	fset := pkg.Fset
	synthesized := synthesizedPackage(pkg.PkgPath)

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if synthesized || decl.Recv == nil || len(decl.Recv.List) == 0 {
					continue
				}
				obj, ok := pkg.TypesInfo.Defs[decl.Name].(*types.Func)
				if !ok {
					continue
				}
				key := declKey(position(fset, obj.Pos()))
				s.concreteMethodSpans[key] = append(s.concreteMethodSpans[key], toSpan(fset, decl.Pos(), decl.End()))
				s.concreteMethodsByName[obj.Name()] = append(s.concreteMethodsByName[obj.Name()], concreteMethod{fn: obj, key: key})
			case *ast.GenDecl:
				switch decl.Tok {
				case token.TYPE:
					if !synthesized {
						s.addInterfaceMethods(pkg, decl)
					}
				case token.VAR:
					s.addBlankVarSpans(pkg, decl)
				}
			}
		}
	}
}

// addPackageUses records the method selections the view admits.
func (s *methodReferenceScan) addPackageUses(pkg *packages.Package, v view) {
	fset := pkg.Fset
	for selector, selection := range pkg.TypesInfo.Selections {
		if selection.Kind() != types.MethodVal && selection.Kind() != types.MethodExpr {
			continue
		}
		obj := selection.Obj()
		if obj == nil || obj.Pkg() == nil {
			continue
		}
		pos := position(fset, selector.Sel.Pos())
		if !v.admitsSelection(pkg.PkgPath, pos.Filename, selection) {
			continue
		}
		key := declKey(position(fset, obj.Pos()))
		if _, ok := s.interfaceMethods[key]; ok {
			s.interfaceMethodUses[key] = append(s.interfaceMethodUses[key], pos)
		} else {
			s.concreteMethodUses[key] = append(s.concreteMethodUses[key], pos)
		}
		if fn, ok := obj.(*types.Func); ok {
			s.addDynamicDispatchUses(fn, pos)
		}
	}
}

// addDynamicDispatchUses records a selection of an interface method
// as a dispatch-conferred use of every concrete method in the module
// that implements that interface and whose receiver the program materializes
// behind an interface somewhere. This models capability dispatch — storing
// a component behind a broad interface and discovering narrower capabilities
// with type assertions — where the selection resolves to the (possibly private
// or anonymous) capability interface's method rather than the concrete
// implementation. Structural satisfaction on its own is not enough: without
// the materialization gate any method would be credited by a same-named
// selection anywhere.
func (s *methodReferenceScan) addDynamicDispatchUses(method *types.Func, pos token.Position) {
	recv := method.Signature().Recv()
	if recv == nil {
		return
	}
	iface, ok := recv.Type().Underlying().(*types.Interface)
	if !ok {
		return
	}
	for _, impl := range s.concreteMethodsByName[method.Name()] {
		if s.evidence.materialized(impl.fn) && implementsInterface(impl.fn, iface) {
			s.dispatched[impl.key] = append(s.dispatched[impl.key], pos)
		}
	}
}

// addInterfaceMethods indexes every interface method the package declares,
// including the ones in test and generated files. The index is what tells
// participation whether an interface belongs to this analysis, and a test
// file's interface is this analysis's: its call sites are in the loaded
// program, unlike a dependency's. Which of them are reported is decided where
// they are reported.
func (s *methodReferenceScan) addInterfaceMethods(pkg *packages.Package, decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		typeSpec := spec.(*ast.TypeSpec)
		ifaceType, ok := typeSpec.Type.(*ast.InterfaceType)
		if !ok {
			continue
		}
		for _, field := range ifaceType.Methods.List {
			if len(field.Names) == 0 {
				continue
			}
			for _, name := range field.Names {
				obj, ok := pkg.TypesInfo.Defs[name].(*types.Func)
				if !ok {
					continue
				}
				pos := position(pkg.Fset, obj.Pos())
				key := declKey(pos)
				s.interfaceMethods[key] = declaration{
					pos: pos, verdict: VerdictUnusedInterfaceMethod,
					name: typeSpec.Name.Name + "." + obj.Name(),
					kind: KindInterfaceMethod, pkg: pkg.PkgPath, exported: obj.Exported(),
				}
				s.interfaceMethodSpans[key] = append(s.interfaceMethodSpans[key], toSpan(pkg.Fset, field.Pos(), field.End()))
			}
		}
	}
}

func (s *methodReferenceScan) addBlankVarSpans(pkg *packages.Package, decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		valueSpec := spec.(*ast.ValueSpec)
		if allBlank(valueSpec.Names) {
			s.blankSpans.add(blankAssertionSpans(pkg, valueSpec)...)
		}
	}
}

func unusedInterfaceMethods(scan *methodReferenceScan, flows *interfaceFlows) []declaration {
	var unused []declaration
	for key, decl := range scan.interfaceMethods {
		if flows.used(key) || scan.generated[decl.pos.Filename] {
			continue
		}
		unused = append(unused, decl)
	}
	sortDeclarations(unused)
	return unused
}

// unusedReflectionLiveMethods reports exported methods that reachability
// considered live only through reflection and that have no source-level method
// use. The distinction is what the two reachability maps are for: a method
// the concrete call graph explains is genuinely called, while one only RTA's
// runtime types keep alive is reachable in the sense that reflection could find
// it and in no other.
func unusedReflectionLiveMethods(
	scan *methodScan,
	reach reachability,
	refs *methodReferenceScan,
	participation func(key string) bool,
) []declaration {
	var unused []declaration
	for key, decl := range scan.declared {
		if !reach.all[key] || reach.graph[key] ||
			participation(key) || refs.concreteMethodUsed(key) {
			continue
		}
		decl.verdict = VerdictReflectionLiveMethod
		unused = append(unused, decl)
	}
	sortDeclarations(unused)
	return unused
}

// unusedExportedMethods reports exported methods no reference reaches.
// The verdict is independent of reachability on purpose: an exported method
// that nothing selects is unused whether or not a root happens to reach its
// receiver, which is what keeps an exported declaration from passing as live
// merely because it was rooted. Methods participation covers are excluded —
// the interface the receiver sits behind is the use.
func unusedExportedMethods(
	scan *methodScan,
	refs *methodReferenceScan,
	reported []declaration,
	participation func(key string) bool,
) []declaration {
	already := make(map[string]bool, len(reported))
	for _, decl := range reported {
		already[declKey(decl.pos)] = true
	}
	var unused []declaration
	for key, decl := range scan.declared {
		if already[key] || participation(key) || refs.concreteMethodUsed(key) {
			continue
		}
		unused = append(unused, decl)
	}
	sortDeclarations(unused)
	return unused
}

// concreteMethodUsed reports a syntactic selection of the declaration itself,
// outside its own extent and blank assertions.
func (s *methodReferenceScan) concreteMethodUsed(key string) bool {
	return usedOutsideSpans(s.concreteMethodUses[key], s.concreteMethodSpans[key], s.blankSpans)
}

// dispatchCredited reports a use dynamic dispatch confers on the declaration,
// filtered the same way a direct selection is.
func (s *methodReferenceScan) dispatchCredited(key string) bool {
	return usedOutsideSpans(s.dispatched[key], s.concreteMethodSpans[key], s.blankSpans)
}

func (s *methodReferenceScan) interfaceMethodUsed(key string) bool {
	return usedOutsideSpans(s.interfaceMethodUses[key], s.interfaceMethodSpans[key], s.blankSpans)
}
