package engine

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
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
// a root is — and so are the credited methods of what the sealed surface
// holds, whose bodies run from a call RTA cannot see. Neither counts toward
// having roots at all: a program with no entry point runs none of them.
//
// Under the masked view neither test-variant packages nor declared test-facing
// packages contribute roots of any kind. The synthesized test main
// is an ordinary main package, and the in-package variant shares the plain
// package's path while its initializer runs the _test.go files' inits, so
// exclusion goes by package identity — only the plain variants remain.
// Instantiations and credited methods are admitted by declaration instead,
// for the reasons instantiatedRoots and creditedMethods give.
func selectRoots(
	prog *ssa.Program,
	ssaPkgs []*ssa.Package,
	initial []*packages.Package,
	surface *apiSurface,
	sealed *sealedSurface,
	deadTypes map[string]bool,
	facts map[*packages.Package]fileFacts,
	tests bool,
	v view,
	participation func(key string) bool,
) ([]*ssa.Function, []declaration, error) {
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
	var unmeasured []declaration
	var settled map[string]bool
	if surface.rootsInstantiations() {
		roots = append(roots, instantiatedRoots(prog, surface, deadTypes)...)
		settled = instantiatedOrigins(prog)
		var reached []*ssa.Function
		reached, unmeasured = uninstantiatedRoots(prog, surfaceGenerics(prog, ssaPkgs, surface, deadTypes), settled)
		roots = append(roots, reached...)
	}
	if len(roots) == 0 {
		for i := range ssaPkgs {
			roots = append(roots, exportedRoots(prog, rooted(i), deadTypes)...)
		}
	}
	if len(roots) == 0 {
		return nil, nil, ErrNoRoots
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
	concrete, generic := creditedMethods(prog, sealed, participation, v)
	roots = append(roots, concrete...)
	if surface.rootsInstantiations() {
		reached, more := uninstantiatedRoots(prog, generic, settled)
		roots = append(roots, reached...)
		unmeasured = append(unmeasured, more...)
	}
	sortDeclarations(unmeasured)
	return roots, unmeasured, nil
}

// creditedMethods returns the methods participation credits on the types
// the sealed surface holds, split into those RTA can root as they stand
// and generic ones it cannot. A consumer puts such a type behind a sealed
// interface in code this analysis does not load, so RTA never learns
// it as a runtime type and reaches none of its methods, not even through a call
// it can see. The credit spares the method's own verdict; rooting it reaches
// what the method calls, which would otherwise be reported on exactly the claim
// the method was spared.
//
// A method credited on evidence the program carries takes no root. Where
// that evidence runs, RTA sees the conversion and reaches the method through
// the runtime type it makes; where nothing runs it, the credit spares
// the method's verdict and no more. Rooting it would carry into the live set
// code that only an unreached bind names — in the masked view, code that only
// tests reach.
//
// A method of a generic type roots through the concrete instantiations
// the program builds of it, wherever they were written, on the terms
// instantiatedRoots gives the surface's own generics. Its generic form comes
// back as well, for the caller to walk when it has none; it is found through
// its declaration, since no function set x/tools yields holds a method
// of a generic type until something instantiates it. A method the view does
// not admit roots nothing: under the masked view its body is test code.
func creditedMethods(
	prog *ssa.Program,
	sealed *sealedSurface,
	participation func(key string) bool,
	v view,
) (concrete, generic []*ssa.Function) {
	credited := func(fn *ssa.Function) bool {
		return fn.Pos().IsValid() && v.admitsFunction(prog.Fset, fn) &&
			participation(declKey(position(prog.Fset, fn.Pos())))
	}
	for fn := range ssautil.AllFunctions(prog) {
		if !sealed.holds(fn) || !credited(fn) {
			continue
		}
		if fn.TypeParams().Len() == 0 || concreteInstance(fn) {
			concrete = append(concrete, fn)
		}
	}
	for typ := range sealed.held {
		named, ok := typ.(*types.Named)
		if !ok || named.TypeParams().Len() == 0 {
			continue
		}
		for _, method := range declaredMethods(prog, named) {
			if credited(method) {
				generic = append(generic, method)
			}
		}
	}
	return concrete, generic
}

// declaredMethods returns the functions of the methods declared on named, found
// through their declarations: x/tools builds no method value
// for a parameterized receiver or a generic method, so a method set yields
// neither.
func declaredMethods(prog *ssa.Program, named *types.Named) []*ssa.Function {
	var methods []*ssa.Function
	for method := range named.Methods() {
		if fn := prog.FuncValue(method); fn != nil {
			methods = append(methods, fn)
		}
	}
	return methods
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
	sealed *sealedSurface,
	deadTypes map[string]bool,
	facts map[*packages.Package]fileFacts,
	tests bool,
	v view,
	participation func(key string) bool,
) ([]declaration, []declaration, reachability, error) {
	roots, unmeasured, err := selectRoots(
		prog, ssaPkgs, initial, surface, sealed, deadTypes, facts, tests, v, participation,
	)
	if err != nil {
		return nil, nil, reachability{}, err
	}
	if err = canceled(ctx, "analyzing reachability"); err != nil {
		return nil, nil, reachability{}, err
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
	return found, unmeasured, reach, nil
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
// trivially alive. Generic functions, generic named types and generic methods
// are skipped: an uninstantiated body has no concrete instance to root at,
// and RTA panics when asked to register a type parameter as a runtime type.
// Code a generic API alone reaches is therefore reported as unreachable —
// the cost of measuring liveness against a generic public API, and what
// [GenericRootingInstantiated] answers by rooting instantiations instead.
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

// instantiatedRoots returns the concrete instantiations the program builds
// of a declared API package's exported generic surface. A generic declaration
// cannot be a root itself — see exportedRoots — while every instantiation
// the builder monomorphizes is an ordinary concrete function, so rooting those
// is what makes the code beneath a generic API measurable.
//
// Which type arguments the program supplies is not read as evidence about
// the API. An instantiation's body calls the same declarations whichever
// argument a consumer picks, and the body is what reachability is taken from;
// the argument decides only which of its own methods become runtime types.
// So instantiations are gathered from the whole program rather than from
// the files a view admits: for a library, the only code instantiating its
// public generics is usually its own tests, and refusing those would leave
// the surface unrooted in exactly the view the question is asked in.
func instantiatedRoots(prog *ssa.Program, surface *apiSurface, deadTypes map[string]bool) []*ssa.Function {
	var roots []*ssa.Function
	for fn := range ssautil.AllFunctions(prog) {
		origin := fn.Origin()
		if origin == nil || origin == fn || !concreteInstance(fn) {
			continue
		}
		obj := origin.Object()
		if obj == nil || !obj.Exported() {
			continue
		}
		pkg := obj.Pkg()
		if pkg == nil || !surface.packages[pkg.Path()] || !exportedRootPackage(pkg.Path()) {
			continue
		}
		// A method is rooted on the same terms exportedRoots gives one: only while
		// its receiver type is exported and still referenced.
		if named := receiverOrigin(origin); named != nil {
			if !named.Obj().Exported() || deadTypes[declKey(position(prog.Fset, named.Obj().Pos()))] {
				continue
			}
		}
		roots = append(roots, fn)
	}
	return roots
}

// concreteInstance reports whether an instantiation substituted every type
// parameter for a type. A generic called from inside another generic records
// the enclosing declaration's parameters as its arguments, and the function
// that results still carries free ones — RTA panics the moment it has
// to register one as a runtime type.
//
// [ssa.Function.TypeParams] is not the test. On an instantiation it reports
// the parameters that were substituted rather than any left free, so
// it is non-empty for every instantiation and rejects the whole set.
func concreteInstance(fn *ssa.Function) bool {
	args := fn.TypeArgs()
	return len(args) > 0 && !slices.ContainsFunc(args, parametricType)
}

// receiverOrigin returns the generic named type a method is declared on, or nil
// for a free function.
func receiverOrigin(fn *ssa.Function) *types.Named {
	recv := fn.Signature.Recv()
	if recv == nil {
		return nil
	}
	receiver := types.Unalias(recv.Type())
	if pointer, ok := receiver.(*types.Pointer); ok {
		receiver = types.Unalias(pointer.Elem())
	}
	named, ok := receiver.(*types.Named)
	if !ok {
		return nil
	}
	return named.Origin()
}

// instantiatedOrigins returns the generic declarations the program builds
// a concrete instance of. Origins are keyed by declaration, not by function:
// a package and its test variant carry distinct functions for one source
// declaration, and pointer identity would call a generic instantiated under one
// of them uninstantiated under the other.
func instantiatedOrigins(prog *ssa.Program) map[string]bool {
	instantiated := map[string]bool{}
	for fn := range ssautil.AllFunctions(prog) {
		if origin := fn.Origin(); origin != nil && origin != fn && concreteInstance(fn) {
			instantiated[declKey(position(prog.Fset, origin.Pos()))] = true
		}
	}
	return instantiated
}

// uninstantiatedRoots covers the generic entry points no instance roots:
// declarations nothing in the loaded program ever wrote a concrete argument
// vector for, so there is no monomorphized body to hand RTA. The origin body
// still exists, and the calls it makes to concrete functions are the same calls
// whatever a consumer instantiates it with, so those callees are rooted
// directly and RTA carries on from them with its usual precision. A callee only
// parametrically instantiated has the same problem and is walked in turn.
//
// A call the walk cannot resolve — through an interface, or through a function
// value — needs the type flow only a real instantiation carries. Rather than
// let the declarations past it be reported as unreachable on the strength
// of an edge that was never followed, the generic is returned as a finding
// of its own: the run says it could not measure this, which is a different
// claim from saying what lies beyond it is dead.
//
// settled holds the declarations already covered — every one the program
// instantiates, and every one an earlier call walked — and the call records
// the ones it walks there, so a generic that arrives twice, on the surface
// and credited or once per package variant, is walked and reported once.
func uninstantiatedRoots(
	prog *ssa.Program,
	generics []*ssa.Function,
	settled map[string]bool,
) ([]*ssa.Function, []declaration) {
	var roots []*ssa.Function
	var unmeasured []declaration
	for _, generic := range generics {
		key := declKey(position(prog.Fset, generic.Pos()))
		if settled[key] {
			continue
		}
		settled[key] = true
		reached, followed := walkGenericBody(generic)
		roots = append(roots, reached...)
		if !followed {
			unmeasured = append(unmeasured, unmeasuredDeclaration(prog, generic))
		}
	}
	return roots, unmeasured
}

// walkGenericBody collects the concrete functions an uninstantiated generic's
// body calls, walking through the generics it calls parametrically, and reports
// whether every call it met resolved. A built-in is not a call to a declaration
// and so is not a gap.
func walkGenericBody(generic *ssa.Function) ([]*ssa.Function, bool) {
	var reached []*ssa.Function
	followed := true
	seen := map[*ssa.Function]bool{}
	var walk func(fn *ssa.Function)
	walk = func(fn *ssa.Function) {
		if fn == nil || seen[fn] {
			return
		}
		seen[fn] = true
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(ssa.CallInstruction)
				if !ok {
					continue
				}
				common := call.Common()
				callee := common.StaticCallee()
				switch {
				case callee == nil:
					if _, builtin := common.Value.(*ssa.Builtin); !builtin {
						followed = false
					}
				case callee.TypeParams().Len() == 0 || concreteInstance(callee):
					reached = append(reached, callee)
				default:
					walk(callee)
				}
			}
		}
	}
	walk(generic)
	return reached, followed
}

// unmeasuredDeclaration renders a generic the walk could not follow
// as the finding that says so.
func unmeasuredDeclaration(prog *ssa.Program, generic *ssa.Function) declaration {
	name := generic.Name()
	kind := KindFunc
	owner := ""
	if named := receiverOrigin(generic); named != nil {
		kind = KindMethod
		name = named.Obj().Name() + "." + name
		owner = declKey(position(prog.Fset, named.Obj().Pos()))
	}
	pkg := ""
	if obj := generic.Object(); obj != nil && obj.Pkg() != nil {
		pkg = obj.Pkg().Path()
	}
	return declaration{
		pos:      position(prog.Fset, generic.Pos()),
		verdict:  VerdictUnmeasuredGeneric,
		name:     name,
		kind:     kind,
		pkg:      pkg,
		exported: token.IsExported(generic.Name()),
		owner:    owner,
	}
}

// surfaceGenerics returns the declarations exportedRoots had to skip across
// the declared surface: each surface package's exported generic functions,
// and the exported generic methods of its exported named types.
func surfaceGenerics(
	prog *ssa.Program,
	ssaPkgs []*ssa.Package,
	surface *apiSurface,
	deadTypes map[string]bool,
) []*ssa.Function {
	var found []*ssa.Function
	for _, pkg := range ssaPkgs {
		if pkg == nil || !surface.packages[pkg.Pkg.Path()] || !exportedRootPackage(pkg.Pkg.Path()) {
			continue
		}
		found = append(found, exportedGenerics(prog, pkg, deadTypes)...)
	}
	return found
}

// exportedGenerics returns one package's exported generic functions,
// and the exported generic methods of its exported named types: every method
// of a generic type, and a method declaring type parameters of its own
// on any type.
func exportedGenerics(prog *ssa.Program, pkg *ssa.Package, deadTypes map[string]bool) []*ssa.Function {
	var found []*ssa.Function
	for name, member := range pkg.Members {
		if !token.IsExported(name) {
			continue
		}
		switch member := member.(type) {
		case *ssa.Function:
			if member.TypeParams().Len() > 0 {
				found = append(found, member)
			}
		case *ssa.Type:
			named, ok := member.Type().(*types.Named)
			if !ok || deadTypes[declKey(position(prog.Fset, member.Object().Pos()))] {
				continue
			}
			for _, method := range declaredMethods(prog, named) {
				if token.IsExported(method.Name()) && method.TypeParams().Len() > 0 {
					found = append(found, method)
				}
			}
		}
	}
	return found
}
