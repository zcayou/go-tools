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
	"golang.org/x/tools/go/ssa/ssautil"
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
	surface, err := newAPISurface(cfg.API, cfg.APIExempt)
	if err != nil {
		return nil, err
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

	patterns := cfg.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
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
	if err = surface.resolve(load); err != nil {
		return nil, err
	}

	facts := newFileFacts(pkgs)
	methodDecls := newMethodScan(pkgs, facts)

	prog, ssaPkgs := ssautil.Packages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	if err = canceled(ctx, "building ssa"); err != nil {
		return nil, err
	}

	full, err := evaluate(ctx, prog, ssaPkgs, pkgs, surface, facts, methodDecls, cfg.Tests, view{})
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
	masked, err := evaluate(ctx, prog, ssaPkgs, pkgs, surface, facts, methodDecls, false, view{masked: true})
	if err != nil {
		return nil, fmt.Errorf("analyzing test-only liveness: %w", err)
	}
	return append(findings, testOnlyFindings(full, masked)...), nil
}

// evaluate runs every verdict pass over the built program under one evidence
// view and returns the surface-filtered findings in report order.
func evaluate(
	ctx context.Context,
	prog *ssa.Program,
	ssaPkgs []*ssa.Package,
	pkgs []*packages.Package,
	surface *apiSurface,
	facts map[*packages.Package]fileFacts,
	methodDecls *methodScan,
	tests bool,
	v view,
) ([]declaration, error) {
	// The reference scans run first: whether a type is referenced decides both
	// which API methods stay exempt and which of them are worth rooting.
	idents := unusedExportedIdents(pkgs, facts, v)
	deadTypes := unreferencedTypes(idents, surface)

	// Whether the program can hold a type behind an interface is evidence every
	// credit gate needs, so it is derived once from the built program —
	// with the generic instantiations resolved first, because reflect.TypeFor's
	// resolved type arguments are closure seeds — and shared.
	inst := newInstantiations(pkgs, v)
	ev := newEvidence(prog, inst, v)
	methodRefs := newMethodReferenceScan(pkgs, facts, ev, v)
	flows := newInterfaceFlows(prog, inst, methodRefs, v)
	binds := newInterfaceBindScan(prog, pkgs, methodRefs, analyzedPackages(pkgs), ev, inst, flows, v)
	if err := canceled(ctx, "scanning interface binds"); err != nil {
		return nil, err
	}

	// participation is the one answer every method verdict reads: a method covered
	// by bind or dispatch evidence is reported by none of them, because each would
	// be asserting the same claim against the same facts.
	participation := func(key string) bool {
		return binds.bound(key) || methodRefs.dispatchCredited(key)
	}

	funcs, reach, err := unreachableFuncs(ctx, prog, ssaPkgs, pkgs, surface, deadTypes, facts, tests, v, participation)
	if err != nil {
		return nil, err
	}
	interfaceMethods := unusedInterfaceMethods(methodRefs, flows)
	reflectionMethods := unusedReflectionLiveMethods(methodDecls, reach, methodRefs, participation)
	methods := unusedExportedMethods(methodDecls, methodRefs, reflectionMethods, participation)

	var found []declaration
	for _, decl := range slices.Concat(funcs, idents, interfaceMethods, reflectionMethods, methods) {
		if surface.exempted(decl, deadTypes) {
			continue
		}
		found = append(found, decl)
	}
	return found, nil
}

// testOnlyFindings diffs the two views by declaration: a production declaration
// the masked view reports and the full view does not is one only test evidence
// keeps alive, and it surfaces under its masked verdicts in their test-only
// form. Declarations in test files are judged in the full view alone — test
// code judging test code has no masked question to answer.
func testOnlyFindings(full, masked []declaration) []Finding {
	reported := make(map[string]bool, len(full))
	for _, decl := range full {
		reported[declKey(decl.pos)] = true
	}
	var findings []Finding
	for _, decl := range masked {
		if reported[declKey(decl.pos)] || testFile(decl.pos.Filename) {
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
// whose own liveness it can measure.
func analyzedPackages(pkgs []*packages.Package) map[string]bool {
	analyzed := make(map[string]bool, len(pkgs))
	for _, pkg := range pkgs {
		if pkg.PkgPath != "" {
			analyzed[pkg.PkgPath] = true
		}
	}
	return analyzed
}
