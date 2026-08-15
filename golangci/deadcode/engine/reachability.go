package engine

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// ErrNoRoots is returned when the loaded packages offer nothing to root at, so
// reachability is undefined rather than empty.
var ErrNoRoots = errors.New("no main packages and no exported declarations among loaded packages")

// reachability is what Rapid Type Analysis established about the program:
// the functions it found reachable at all, and the subset reachable through
// call-graph edges it can explain. A function in the first and not the second
// is live only in the sense that reflection could reach it.
type reachability struct {
	all   map[string]bool
	graph map[string]bool
}

// selectRoots gathers the entry points RTA starts from. Roots come from
// the entry points that exist: main packages, which including tests extends
// with the synthesized test binaries and the test functions cmd/go runs,
// and the exported surface of the declared API packages. Only when neither
// exists does the whole exported surface stand in, so that a library still
// analyzes. Functions a //go:linkname directive publishes are added either way
// — the body runs under a name no source reference mentions, which is what
// a root is.
//
// Under the masked view neither test-variant packages nor declared test-facing
// packages contribute roots of any kind. The synthesized test main
// is an ordinary main package, and the in-package variant shares the plain
// package's path while its initializer runs the _test.go files' inits, so
// exclusion goes by package identity — only the plain variants remain.
func selectRoots(
	prog *ssa.Program,
	ssaPkgs []*ssa.Package,
	initial []*packages.Package,
	surface *apiSurface,
	deadTypes map[string]bool,
	facts map[*packages.Package]fileFacts,
	tests bool,
	v view,
) ([]*ssa.Function, error) {
	rooted := func(i int) *ssa.Package {
		if ssaPkgs[i] == nil || !v.admitsRoots(initial[i]) {
			return nil
		}
		return ssaPkgs[i]
	}

	var roots []*ssa.Function
	for i := range ssaPkgs {
		ssaPkg := rooted(i)
		if ssaPkg == nil {
			continue
		}
		if ssaPkg.Pkg.Name() == "main" && ssaPkg.Func("main") != nil {
			roots = append(roots, packageFuncs(ssaPkg, "init", "main")...)
		}
		if tests {
			roots = append(roots, testEntries(ssaPkg)...)
		}
	}
	for i := range ssaPkgs {
		if ssaPkg := rooted(i); ssaPkg != nil && surface.packages[ssaPkg.Pkg.Path()] {
			roots = append(roots, exportedRoots(prog, ssaPkg, deadTypes)...)
		}
	}
	if len(roots) == 0 {
		for i := range ssaPkgs {
			roots = append(roots, exportedRoots(prog, rooted(i), deadTypes)...)
		}
	}
	if len(roots) == 0 {
		return nil, ErrNoRoots
	}
	for i := range ssaPkgs {
		ssaPkg := rooted(i)
		if ssaPkg == nil {
			continue
		}
		for name := range facts[initial[i]].linknamed {
			roots = append(roots, packageFuncs(ssaPkg, name)...)
		}
	}
	return roots, nil
}

func packageFuncs(pkg *ssa.Package, names ...string) []*ssa.Function {
	var funcs []*ssa.Function
	for _, name := range names {
		if fn := pkg.Func(name); fn != nil && fn.TypeParams().Len() == 0 {
			funcs = append(funcs, fn)
		}
	}
	return funcs
}

// testEntries returns the testing entry points cmd/go may run. Rooting them
// covers what the synthesized test main leaves out: an example without
// an output comment is compiled but never registered, so nothing reaches
// it or the helpers only it calls through the generated main.
func testEntries(pkg *ssa.Package) []*ssa.Function {
	var entries []*ssa.Function
	for _, member := range pkg.Members {
		fn, ok := member.(*ssa.Function)
		if !ok || fn.Signature.Recv() != nil || fn.TypeParams().Len() != 0 {
			continue
		}
		if !testFile(position(pkg.Prog.Fset, fn.Pos()).Filename) {
			continue
		}
		switch name := fn.Name(); {
		case strings.HasPrefix(name, "Test"), strings.HasPrefix(name, "Benchmark"),
			strings.HasPrefix(name, "Example"), strings.HasPrefix(name, "Fuzz"):
			entries = append(entries, fn)
		}
	}
	return entries
}

// unreachableFuncs reports source functions not reachable from the selected
// roots via Rapid Type Analysis, mirroring x/tools cmd/deadcode. Every loaded
// package is scanned, including any no root imports: RTA already answers
// correctly there — nothing in such a package is reachable, which is the truth
// about it — and restricting the scan to the roots' import closure would
// instead leave an unimported package silently unreported.
//
// The method arm additionally consults participation, and this is a deliberate
// divergence from cmd/deadcode: a method covered by bind or dispatch evidence
// is invoked through an interface RTA cannot see behind — the invoking call
// sits in a dependency body SSA never built, or behind a descriptor
// reflect.TypeFor conjured — so reporting it unreachable would assert what
// the participation verdicts just refused to.
func unreachableFuncs(
	ctx context.Context,
	prog *ssa.Program,
	ssaPkgs []*ssa.Package,
	initial []*packages.Package,
	surface *apiSurface,
	deadTypes map[string]bool,
	facts map[*packages.Package]fileFacts,
	tests bool,
	v view,
	participation func(key string) bool,
) ([]declaration, reachability, error) {
	roots, err := selectRoots(prog, ssaPkgs, initial, surface, deadTypes, facts, tests, v)
	if err != nil {
		return nil, reachability{}, err
	}
	if err = canceled(ctx, "analyzing reachability"); err != nil {
		return nil, reachability{}, err
	}

	res := rta.Analyze(roots, true)

	reach := reachability{all: map[string]bool{}, graph: reachableThroughCallGraph(roots, res, prog.Fset)}
	for fn := range res.Reachable {
		if fn.Pos().IsValid() {
			reach.all[declKey(position(prog.Fset, fn.Pos()))] = true
		}
	}

	found := unreachableDecls(prog, initial, facts, reach, participation)
	sortDeclarations(found)
	return found, reach, nil
}

// unreachableDecls scans the analyzed declarations for function declarations
// reachability does not cover.
func unreachableDecls(
	prog *ssa.Program,
	initial []*packages.Package,
	facts map[*packages.Package]fileFacts,
	reach reachability,
	participation func(key string) bool,
) []declaration {
	seen := map[string]bool{}
	var found []declaration
	for _, pkg := range initial {
		if synthesizedPackage(pkg.PkgPath) {
			continue
		}
		generated := facts[pkg].generated
		linknamed := facts[pkg].linknamed
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				funcDecl, ok := decl.(*ast.FuncDecl)
				if !ok || funcDecl.Body == nil {
					continue
				}
				// Marker interface methods exist only to satisfy an interface; parity
				// with x/tools deadcode.
				if funcDecl.Recv != nil && len(funcDecl.Body.List) == 0 {
					continue
				}
				if funcDecl.Recv == nil && linknamed[funcDecl.Name.Name] {
					continue
				}
				pos := position(prog.Fset, funcDecl.Name.Pos())
				key := declKey(pos)
				if generated[pos.Filename] || reach.all[key] || seen[key] {
					continue
				}
				if funcDecl.Recv != nil && participation(key) {
					continue
				}
				seen[key] = true

				name := funcDecl.Name.Name
				kind := KindFunc
				if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
					kind = KindMethod
					if ident := receiverBase(funcDecl.Recv.List[0].Type); ident != nil {
						name = ident.Name + "." + name
					}
				}
				found = append(found, declaration{
					pos: pos, verdict: VerdictUnreachable, name: name,
					kind: kind, pkg: pkg.PkgPath, exported: token.IsExported(funcDecl.Name.Name),
					owner: receiverTypeKey(pkg, funcDecl),
				})
			}
		}
	}
	return found
}

// exportedRoots returns the entry points a library package exposes: its
// initializer, its exported package-level functions, and the exported methods
// of its exported named types. Internal and test packages contribute nothing —
// no library consumer can reach them, so rooting there would make their code
// trivially alive. Generic functions and generic named types are skipped:
// an uninstantiated body has no concrete instance to root at, and RTA panics
// when asked to register a type parameter as a runtime type. Code a generic API
// alone reaches is therefore reported as unreachable — the cost of measuring
// liveness against a generic public API.
func exportedRoots(prog *ssa.Program, pkg *ssa.Package, deadTypes map[string]bool) []*ssa.Function {
	if pkg == nil || !exportedRootPackage(pkg.Pkg.Path()) {
		return nil
	}
	roots := packageFuncs(pkg, "init")
	for name, member := range pkg.Members {
		if !token.IsExported(name) {
			continue
		}
		switch member := member.(type) {
		case *ssa.Function:
			if member.TypeParams().Len() == 0 {
				roots = append(roots, member)
			}
		case *ssa.Type:
			// A type no reference reaches keeps nothing alive: its methods are dead
			// with it, so rooting them would hide their callees too.
			if deadTypes[declKey(position(prog.Fset, member.Object().Pos()))] {
				continue
			}
			roots = append(roots, exportedMethods(prog, member.Type())...)
		}
	}
	return roots
}

func exportedRootPackage(path string) bool {
	if strings.HasSuffix(path, ".test") || strings.HasSuffix(path, "_test") {
		return false
	}
	for element := range strings.SplitSeq(path, "/") {
		if element == "internal" {
			return false
		}
	}
	return true
}

func exportedMethods(prog *ssa.Program, receiver types.Type) []*ssa.Function {
	if named, ok := receiver.(*types.Named); ok && named.TypeParams().Len() > 0 {
		return nil
	}
	var methods []*ssa.Function
	for _, form := range []types.Type{receiver, types.NewPointer(receiver)} {
		set := prog.MethodSets.MethodSet(form)
		for selection := range set.Methods() {
			if !selection.Obj().Exported() {
				continue
			}
			if method := prog.MethodValue(selection); method != nil {
				methods = append(methods, method)
			}
		}
	}
	return methods
}

// reachableThroughCallGraph walks the concrete call graph from the roots.
// The traversal is memoized on the function rather than on its source position:
// a synthesized function has no position, and every instantiation of a generic
// function shares the position of the declaration it came from, so
// a position-keyed memo would both abort at every package initializer and visit
// only the first instantiation.
func reachableThroughCallGraph(roots []*ssa.Function, res *rta.Result, fset *token.FileSet) map[string]bool {
	reachable := map[string]bool{}
	seen := map[*ssa.Function]bool{}
	var visit func(fn *ssa.Function)
	visit = func(fn *ssa.Function) {
		if fn == nil || seen[fn] {
			return
		}
		seen[fn] = true
		if fn.Pos().IsValid() {
			reachable[declKey(position(fset, fn.Pos()))] = true
		}
		node := res.CallGraph.Nodes[fn]
		if node == nil {
			return
		}
		for _, edge := range node.Out {
			visit(edge.Callee.Func)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	return reachable
}
