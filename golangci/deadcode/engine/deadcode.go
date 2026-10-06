package engine

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"os"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// loadMode is what the analysis needs of the loader. NeedDeps together
// with NeedSyntax and NeedTypesInfo is load-bearing rather than generous:
// an assertion in a dependency's own source is the only evidence that a library
// invokes an implementation, so dependency syntax has to be present. It also
// makes the load expensive, because every dependency is then read from source
// and its function bodies type checked.
const loadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
	packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedTypesSizes

// Kind classifies a finding by the sort of declaration it names. It decides
// what a declared API surface exempts, which turns on what a consumer can reach
// rather than on the wording of the verdict.
type Kind string

const (
	// KindFunc is a package-level function, whether or not it is exported.
	KindFunc Kind = "func"
	// KindMethod is a method on a concrete type, whether or not it is exported.
	KindMethod Kind = "method"
	// KindType is a package-level named type.
	KindType Kind = "type"
	// KindConst is a package-level constant.
	KindConst Kind = "const"
	// KindVar is a package-level variable.
	KindVar Kind = "var"
	// KindInterfaceMethod is a method an interface declares, which no concrete
	// declaration backs.
	KindInterfaceMethod Kind = "interface-method"
)

// Kinds returns every [Kind], for callers validating an exemption list.
func Kinds() []Kind {
	return []Kind{KindFunc, KindMethod, KindType, KindConst, KindVar, KindInterfaceMethod}
}

// GenericRooting is how a declared API package's generic declarations enter
// the root set. The zero value reads as [GenericRootingInstantiated] wherever
// an API is declared, and is inert everywhere else.
type GenericRooting string

const (
	// GenericRootingSkip leaves generic declarations out of the root set, so
	// what a generic API alone reaches is reported as unreachable, and lets
	// no test's instantiation stand in for a consumer's in the masked view.
	GenericRootingSkip GenericRooting = "skip"
	// GenericRootingInstantiated roots the concrete instantiations the program
	// builds of that surface, which is what makes the code beneath a generic API
	// measurable at all, and weighs its sealed generic contracts against the same
	// instantiations in both views.
	GenericRootingInstantiated GenericRooting = "instantiated"
)

// GenericRootings returns every [GenericRooting], for callers validating one.
func GenericRootings() []GenericRooting {
	return []GenericRooting{GenericRootingSkip, GenericRootingInstantiated}
}

// Verdict is why a declaration was reported. The set is closed: every [Finding]
// carries one of these, and one declaration can draw more than one. Under
// [Config.Tests] each verdict also exists in a test-only form — the same token
// prefixed with "test-only" — carried by a production declaration that only
// test code keeps alive.
type Verdict string

// testOnly derives the test-only form of the verdict, the one a declaration
// draws when the masked view reports it and the full view does not.
func (v Verdict) testOnly() Verdict {
	return "test-only " + v
}

const (
	// VerdictUnreachable is a function no path from any root reaches.
	VerdictUnreachable Verdict = "unreachable func"
	// VerdictUnusedFunc is an exported function nothing outside its own body
	// references.
	VerdictUnusedFunc Verdict = "unused exported func"
	// VerdictUnusedType is an exported type nothing outside its own declaration
	// references.
	VerdictUnusedType Verdict = "unused exported type"
	// VerdictUnusedConst is an exported constant nothing references.
	VerdictUnusedConst Verdict = "unused exported const"
	// VerdictUnusedVar is an exported variable nothing references.
	VerdictUnusedVar Verdict = "unused exported var"
	// VerdictUnusedMethod is an exported method nothing selects and no interface
	// bind credits.
	VerdictUnusedMethod Verdict = "unused exported method"
	// VerdictUnusedInterfaceMethod is a method an interface declares that nothing
	// selects.
	VerdictUnusedInterfaceMethod Verdict = "unused interface method"
	// VerdictReflectionLiveMethod is an exported method reachability keeps alive
	// only through reflection, with no source-level use behind it.
	VerdictReflectionLiveMethod Verdict = "unused reflection-live exported method"
	// VerdictUnmeasuredGeneric is a generic entry point — a declaration
	// on the declared surface, or a method participation credits —
	// that the program never instantiates and whose body makes a call the analysis
	// cannot resolve. It claims nothing about the declaration: it says the run
	// could not follow what the declaration reaches, so an unreachable verdict
	// anywhere past that call would be a guess.
	VerdictUnmeasuredGeneric Verdict = "unmeasured generic"
)

// Config is one analysis request. The zero value analyzes ./... in the working
// directory, rooted at whatever entry points it finds, with no declared API.
type Config struct {
	// Dir is the directory the load runs in, and the directory relative patterns
	// resolve against. Empty means the process working directory, which nothing
	// else in this configuration pins: a caller that means a particular module
	// should say so.
	Dir string

	// Patterns are the package patterns to analyze. Empty means ./...
	//
	// Narrowing this narrows what the analysis can see rather than only what
	// it reports, so it risks calling a live declaration dead: a caller outside
	// the pattern is not loaded, and an unloaded caller looks like no caller. Set
	// it only when one module holds several independent programs.
	Patterns []string

	// BuildTags are extra build tags for the load.
	BuildTags []string

	// Tests includes test code: test binaries become call-graph roots
	// and test-only references count as uses. It also turns on the second, masked
	// evaluation behind the test-only verdict family — a production declaration
	// that only test evidence keeps alive is reported rather than passing as live.
	Tests bool

	// API are package patterns whose exported surface consumers reach.
	API []string

	// APIExempt is the declaration kinds an API package exempts. It requires API.
	// An exempt declaration is still a call-graph root, so whatever it alone
	// reaches stays live.
	APIExempt []Kind

	// APIGenerics is how the declared surface's generic declarations are rooted,
	// and whether a test's instantiation stands in for a consumer's in the masked
	// view. It requires API, and defaults to [GenericRootingInstantiated]. Set
	// it to [GenericRootingSkip] to root only what Rapid Type Analysis takes
	// directly, which reports whatever a generic API alone reaches as unreachable.
	APIGenerics GenericRooting

	// APITestConsumers counts the declared API packages' tests as stand-ins
	// for their consumers in the masked view. Test-origin evidence aimed
	// at the surface, as far as a consumer could have written it, is admitted
	// there as production evidence would be: a reference to an exported
	// declaration of an API package, a selection of an exported method through
	// an exported API type, a conversion with an exported API type at one end
	// and only types a consumer can name at both, and, unless APIGenerics
	// is [GenericRootingSkip], an instantiation of an API generic with such type
	// arguments. Test evidence aimed anywhere else stays masked. A contract test
	// code declares then confers the way a dependency's does, standing
	// in for a consumer's own interface over a public type. Requires API
	// and Tests.
	APITestConsumers bool

	// VocabularyNames credits each closed vocabulary's published name: a String()
	// string method on a value receiver, on a named type whose own package
	// declares typed constants of it, counts as used and is a root. It states
	// a repository's convention — every vocabulary member has a name, whether
	// or not production renders one — rather than evidence the program carries, so
	// it is off by default. Other methods of such a type, and a String on a type
	// no constant is declared of, are judged as ever.
	VocabularyNames bool

	// TestFacing are package patterns whose intended consumers are tests:
	// production-shaped code that exists to be exercised by test files. They join
	// the masked view's definition of test origin — their evidence and roots
	// are removed alongside _test.go facts, their own declarations never draw
	// test-only verdicts, and a production declaration only they keep alive draws
	// the complete test-only family. Requires Tests.
	TestFacing []string

	// Roots are file paths or globs, relative to Dir, naming entry-point files
	// the load cannot reach — conventionally single-file package main programs
	// behind //go:build ignore, run with go run. Each resolved file is loaded
	// as its own main program: its main roots the call graph in both views and its
	// facts count as evidence, while nothing in it is ever reported.
	Roots []string
}

// Finding is one reported declaration.
type Finding struct {
	// Pos is where the declaration's name appears. It is a resolved
	// [token.Position] rather than a token.Pos so that it outlives
	// the [token.FileSet] it was produced in, which is what lets a caller
	// in another FileSet — a go/analysis driver, say — place it.
	Pos token.Position

	// Verdict is why the declaration was reported.
	Verdict Verdict

	// Name is the declaration, qualified by receiver or interface for a method.
	Name string

	// Kind is the sort of declaration it is.
	Kind Kind

	// Package is the import path that declares it.
	Package string
}

// Analyze reports the declarations nothing in the loaded program can reach.
//
// Findings arrive grouped by verdict in a stable order — unreachable functions,
// unused exported identifiers, unused interface methods, reflection-live
// methods, then unused exported methods — and sorted by position within each
// group. Under [Config.Tests] the same groups follow once more in their
// test-only form, holding the production declarations only test evidence keeps
// alive. Callers that want a single positional order must sort.
func Analyze(ctx context.Context, cfg Config) ([]Finding, error) {
	surface, err := newAPISurface(cfg.API, cfg.APIExempt, cfg.APIGenerics)
	if err != nil {
		return nil, err
	}
	declared, err := newTestFacing(cfg.TestFacing, cfg.Tests)
	if err != nil {
		return nil, err
	}
	if cfg.APITestConsumers && (len(surface.patterns) == 0 || !cfg.Tests) {
		// The masked view is all the setting speaks to, and the surface is what
		// it admits evidence toward: without either there is nothing to stand in for.
		return nil, errors.New("api test consumers require api patterns and tests")
	}

	dir := cfg.Dir
	if dir == "" {
		if dir, err = os.Getwd(); err != nil {
			return nil, fmt.Errorf("getting working directory: %w", err)
		}
	}

	load := &packages.Config{
		Mode:    loadMode,
		Context: ctx,
		Dir:     dir,
		Tests:   cfg.Tests,
	}
	if len(cfg.BuildTags) > 0 {
		load.BuildFlags = []string{"-tags=" + strings.Join(cfg.BuildTags, ",")}
	}

	pkgs, err := loadProgram(ctx, cfg, dir, load)
	if err != nil {
		return nil, err
	}
	if err = surface.resolve(load); err != nil {
		return nil, err
	}
	if err = declared.resolve(load, surface); err != nil {
		return nil, err
	}

	facts := newFileFacts(pkgs)
	methodDecls := newMethodScan(pkgs, facts)
	// Every instantiation the program writes, test files included. The full view
	// reads it as its own, and under instantiated rooting the sealed surface reads
	// it in both: which type argument a consumer picks says nothing about who
	// holds the interface.
	whole := newInstantiations(pkgs, view{})
	sealed := newSealedSurface(surface, whole, pkgs)
	vocab := newVocabulary(pkgs, cfg.VocabularyNames)
	copies := newReexports(pkgs, facts, surface)

	prog, ssaPkgs, err := buildProgram(pkgs)
	if err != nil {
		return nil, err
	}
	if err = canceled(ctx, "building ssa"); err != nil {
		return nil, err
	}

	full, err := evaluate(ctx, prog, ssaPkgs, pkgs, surface, facts, methodDecls, copies, whole, sealed, vocab, cfg.Tests, view{})
	if err != nil {
		return nil, err
	}
	findings := make([]Finding, 0, len(full))
	for _, decl := range full {
		findings = append(findings, newFinding(decl))
	}
	if !cfg.Tests {
		return findings, nil
	}

	// The masked view answers the same questions with test-origin evidence
	// removed, and its failures fail the run like any other evaluation's:
	// a program whose only entry points are tests has nothing to root at once they
	// are set aside, and [ErrNoRoots] holds that reachability is then undefined
	// rather than empty — an empty family would read as all clear.
	var consumers *testConsumers
	if cfg.APITestConsumers {
		consumers = &testConsumers{
			api:            surface.packages,
			analyzed:       analyzedPackages(pkgs),
			instantiations: surface.rootsInstantiations(),
		}
	}
	mask := maskedView(pkgs, declared.packages, consumers)
	maskedInst := newInstantiations(pkgs, mask)
	// Under skip nothing a test instantiates stands in for a consumer, so
	// the sealed surface's generic contracts are weighed against the view's own.
	if !surface.rootsInstantiations() {
		sealed = newSealedSurface(surface, maskedInst, pkgs)
	}
	masked, err := evaluate(ctx, prog, ssaPkgs, pkgs, surface, facts, methodDecls, copies, maskedInst, sealed, vocab, false, mask)
	if err != nil {
		return nil, fmt.Errorf("analyzing test-only liveness: %w", err)
	}
	return append(findings, testOnlyFindings(full, masked, declared.packages)...), nil
}

// loadProgram loads the analyzed program: the configured patterns, plus one
// synthesized package per declared root program, validated as a whole.
func loadProgram(ctx context.Context, cfg Config, dir string, load *packages.Config) ([]*packages.Package, error) {
	patterns := cfg.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	var programs []rootProgram
	if len(cfg.Roots) > 0 {
		var overlay map[string][]byte
		var err error
		if programs, overlay, err = loadRootPrograms(dir, cfg.Roots); err != nil {
			return nil, err
		}
		load.Overlay = overlay
		for _, program := range programs {
			patterns = append(patterns, program.pattern)
		}
	}

	pkgs, err := packages.Load(load, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}
	if err = canceled(ctx, "loading packages"); err != nil {
		return nil, err
	}
	if err = loadErrors(pkgs); err != nil {
		return nil, err
	}
	if err = validateRootPrograms(pkgs, programs); err != nil {
		return nil, err
	}
	return pkgs, nil
}

// evaluate runs every verdict pass over the built program under one evidence
// view and returns the surface-filtered findings in report order. inst
// is the instantiations the view admits, sealed the sealed surface it weighs,
// and vocab the published names it credits.
func evaluate(
	ctx context.Context,
	prog *program,
	ssaPkgs []*ssa.Package,
	pkgs []*packages.Package,
	surface *apiSurface,
	facts map[*packages.Package]fileFacts,
	methodDecls *methodScan,
	copies reexports,
	inst *instantiations,
	sealed *sealedSurface,
	vocab *vocabulary,
	tests bool,
	v view,
) ([]declaration, error) {
	// The reference scans run first: whether a type is referenced decides both
	// which API methods stay exempt and which of them are worth rooting.
	idents := unusedExportedIdents(pkgs, facts, copies, v)
	deadTypes := unreferencedTypes(idents, surface)

	// Whether the program can hold a type behind an interface is evidence every
	// credit gate needs, so it is derived once from the built program — seeded
	// from reflect.TypeFor's resolved type arguments and from what the sealed
	// surface holds as well as from conversions — and shared.
	ev := newEvidence(prog, inst, sealed, v)
	methodRefs := newMethodReferenceScan(pkgs, facts, ev, inst, v)
	flows := newInterfaceFlows(prog, inst, methodRefs, v)
	binds := newInterfaceBindScan(prog, pkgs, sealed, methodRefs, analyzedPackages(pkgs), ev, inst, flows, v)
	if err := canceled(ctx, "scanning interface binds"); err != nil {
		return nil, err
	}

	// participation is the one answer every method verdict reads: a method covered
	// by bind or dispatch evidence, or a published name, is reported by none
	// of them, because each would be asserting the same claim against the same
	// facts.
	participation := func(key string) bool {
		return binds.bound(key) || methodRefs.dispatchCredited(key) || vocab.published(key)
	}

	funcs, unmeasured, reach, err := unreachableFuncs(
		ctx, prog, ssaPkgs, pkgs, surface, sealed, vocab, deadTypes, facts, tests, v, participation,
	)
	if err != nil {
		return nil, err
	}
	interfaceMethods := unusedInterfaceMethods(methodRefs, flows)
	reflectionMethods := unusedReflectionLiveMethods(methodDecls, reach, methodRefs, participation)
	methods := unusedExportedMethods(methodDecls, methodRefs, reflectionMethods, participation)

	var found []declaration
	for _, decl := range slices.Concat(funcs, idents, interfaceMethods, reflectionMethods, methods) {
		if surface.exempted(decl, deadTypes) || copies.copies(declKey(decl.pos)) {
			continue
		}
		found = append(found, decl)
	}
	// The unmeasured verdict reports the analysis, not the declaration, so no
	// exemption applies to it: an exempt kind is where a gap in coverage matters
	// most, because nothing else would ever mention that declaration again. A copy
	// is not a declaration at all, so it draws none: the walk past it reaches
	// the original, which answers for itself.
	for _, decl := range unmeasured {
		if !copies.copies(declKey(decl.pos)) {
			found = append(found, decl)
		}
	}
	return found, nil
}

// testOnlyFindings diffs the two views by declaration: a production declaration
// the masked view reports and the full view does not is one only test evidence
// keeps alive, and it surfaces under its masked verdicts in their test-only
// form. Declarations in test files and in declared test-facing packages
// are judged in the full view alone — test code judging test code has no masked
// question to answer, and being alive only through tests is a declared
// package's job.
func testOnlyFindings(full, masked []declaration, testFacing map[string]bool) []Finding {
	reported := make(map[string]bool, len(full))
	for _, decl := range full {
		reported[declKey(decl.pos)] = true
	}
	var findings []Finding
	for _, decl := range masked {
		if reported[declKey(decl.pos)] || testFile(decl.pos.Filename) || testFacing[decl.pkg] {
			continue
		}
		decl.verdict = decl.verdict.testOnly()
		findings = append(findings, newFinding(decl))
	}
	return findings
}

func newFinding(decl declaration) Finding {
	return Finding{
		Pos:     decl.pos,
		Verdict: decl.verdict,
		Name:    decl.name,
		Kind:    decl.kind,
		Package: decl.pkg,
	}
}

// canceled reports the caller's cancellation against the stage it interrupted.
// Every stage after the load runs to completion once entered, so a deadline
// is only observable between them and a result returned without this check
// would be a full answer produced past the deadline.
func canceled(ctx context.Context, stage string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", stage, err)
	}
	return nil
}

// loadErrors reports every error the load recorded, across dependencies as well
// as analyzed packages. A partially loaded program yields findings that cannot
// be trusted — an unparsed caller looks like no caller — so any error fails
// the run rather than degrading it.
func loadErrors(pkgs []*packages.Package) error {
	var errs []error
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, err := range pkg.Errors {
			errs = append(errs, fmt.Errorf("%s: %w", pkg.PkgPath, err))
		}
	})
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("packages contain errors: %w", errors.Join(errs...))
}

// analyzedPackages returns the import paths this run reports on, the interfaces
// whose own liveness it can measure. A synthesized root program is not among
// them — nothing in it is ever reported — so an interface it declares credits
// unconditionally, the way a dependency's does.
func analyzedPackages(pkgs []*packages.Package) map[string]bool {
	analyzed := make(map[string]bool, len(pkgs))
	for _, pkg := range pkgs {
		if pkg.PkgPath != "" && !synthesizedPackage(pkg.PkgPath) {
			analyzed[pkg.PkgPath] = true
		}
	}
	return analyzed
}
