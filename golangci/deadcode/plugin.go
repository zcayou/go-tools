package deadcode

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

// Name is what golangci-lint knows this linter by: the key under
// linters.settings.custom, the entry in linters.enable, and the token
// a //nolint:deadcode directive must carry.
const Name = "deadcode"

// Settings is the decoded linters.settings.custom.deadcode.settings block.
// Decoding rejects keys it does not know, so an unrecognized key fails the run.
type Settings struct {
	// Patterns are the package patterns to analyze, resolved against the module
	// root rather than the working directory. Omit the key to analyze ./..., which
	// is the whole module; an explicitly empty list is rejected rather than read
	// as the default, because a list written out means the packages in it.
	//
	// Narrowing this does not narrow what is reported — reporting is scoped
	// to what golangci-lint was asked to lint — it narrows what the analysis can
	// see, and so risks calling a live declaration dead. Set it only when one
	// module holds several independent programs.
	Patterns []string `json:"patterns"`

	// BuildTags are extra build tags. golangci-lint's run.build-tags cannot
	// be read from a plugin, so it has to be repeated here.
	BuildTags []string `json:"build-tags"`

	// Tests includes test code: test binaries become call-graph roots
	// and test-only references count as uses. It also turns on the test-only
	// verdict family — a production declaration only test code keeps alive
	// is reported with its verdicts prefixed test-only rather than passing
	// as live. Defaults to true, matching golangci-lint's run.tests, which this
	// cannot read.
	Tests *bool `json:"tests"`

	// API are package patterns whose exported surface consumers reach.
	API []string `json:"api"`

	// APIExempt is the declaration kinds an API package exempts: func, method,
	// type, const, var, or interface-method. Omit the key to exempt method;
	// an explicitly empty list exempts nothing, which holds every exported
	// declaration of the surface to a reference. Requires API.
	APIExempt []string `json:"api-exempt"`

	// APIGenerics is how the declared surface's generic declarations are rooted:
	// instantiated, the default, which roots the concrete instantiations
	// the program builds of them and weighs the surface's sealed generic contracts
	// against the same ones in both views, or skip, which roots none of them
	// and so reports whatever a generic API alone reaches as unreachable. Requires
	// API.
	APIGenerics *string `json:"api-generics"`

	// APITestConsumers counts the api packages' tests as stand-ins for their
	// consumers, which this run cannot see. In the test-only family's masked view,
	// test evidence aimed at the declared surface counts as production evidence
	// would, as far as a consumer could have written it: a reference
	// to an exported declaration of an api package, a selection of an exported
	// method through an exported api type, a conversion with an exported api type
	// at one end and only types a consumer can name at both, and, unless
	// api-generics is skip, an instantiation of an api generic with such type
	// arguments. An interface a test declares confers the way a consumer's own
	// would. Test evidence aimed anywhere else stays masked, so production code
	// off the surface that only tests keep alive still draws the family. Requires
	// api and tests.
	APITestConsumers bool `json:"api-test-consumers"`

	// VocabularyNames credits each closed vocabulary's published name. A named
	// type whose own package declares typed constants of it is a vocabulary,
	// and a String() string method on a value receiver is its name: it counts
	// as used, and what it calls stays live, whether or not production ever
	// renders a member. Other methods of the type, and a String on a type
	// with no declared constant, are judged as ever. Off by default, because
	// it states a convention rather than evidence the program carries.
	VocabularyNames bool `json:"vocabulary-names"`

	// TestFacing are package patterns whose intended consumers are tests:
	// production-shaped code, a test plugin say, that exists to be exercised
	// by test files. A declared package draws no test-only verdicts of its own —
	// being alive only through tests is its job — while a production declaration
	// only it keeps alive draws the complete test-only family. Requires tests.
	TestFacing []string `json:"test-facing"`

	// Roots are file paths or globs, relative to the module root, naming
	// entry-point files the analysis cannot otherwise see — conventionally
	// single-file package main programs behind //go:build ignore, run with go run.
	// Each resolved file is loaded as its own main program: its call edges
	// and references count as production evidence, and nothing in it is ever
	// reported.
	Roots []string `json:"roots"`
}

// Plugin adapts the whole-program engine to golangci-lint's per-package linter
// contract. It holds the single analysis the whole run shares.
type Plugin struct {
	config engine.Config

	once   sync.Once
	byFile map[string][]engine.Finding
	err    error
}

var _ register.LinterPlugin = (*Plugin)(nil)

// New builds the plugin from the raw settings golangci-lint decoded out
// of the configuration file. It satisfies [register.NewPlugin].
//
// It decodes and validates, and touches nothing else. golangci-lint constructs
// every configured plugin whether or not the run enables it, and a failure here
// aborts configuration loading — so anything that can fail on an unrelated
// command, listing linters or formatting among them, has to wait until
// the analysis actually runs.
func New(settings any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](settings)
	if err != nil {
		// register.DecodeSettings already names the step, and golangci-lint prefixes
		// the plugin.
		return nil, err
	}
	config, err := configFrom(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", Name, err)
	}
	return &Plugin{config: config}, nil
}

// BuildAnalyzers returns the analyzer this linter runs, alongside the one
// that keeps its results from being cached.
func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{p.analyzer(), uncacheable()}, nil
}

// uncacheable defeats golangci-lint's per-package issue cache. That cache keys
// on a package and its imports, while dead-code liveness flows the other way,
// from importers: a package whose own hash is unchanged would replay an answer
// the edit that changed it never invalidated, silently keeping a finding
// that is now wrong or losing one that is now right. golangci-lint folds every
// analyzer name into the cache key, so a name unique to this process forces
// a miss for every package.
func uncacheable() *analysis.Analyzer {
	var nonce [8]byte
	_, _ = rand.Read(nonce[:])
	return &analysis.Analyzer{
		Name: fmt.Sprintf("%s_uncacheable_%x", Name, nonce),
		Doc:  "forces a cache miss; reports nothing",
		Run:  func(*analysis.Pass) (any, error) { return nil, nil },
	}
}

// GetLoadMode asks for syntax only. The analyzer reads nothing but file names
// and positions from the pass — the engine does its own typed load — so making
// golangci-lint type check on this linter's behalf would be wasted work.
func (p *Plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}

// configFrom validates the settings.
func configFrom(s Settings) (engine.Config, error) {
	exempts, err := exemptions(s)
	if err != nil {
		return engine.Config{}, err
	}
	generics, err := genericRooting(s)
	if err != nil {
		return engine.Config{}, err
	}
	if s.Patterns != nil && len(s.Patterns) == 0 {
		return engine.Config{}, errors.New("patterns is empty: omit it to analyze the whole module")
	}
	if s.TestFacing != nil && len(s.TestFacing) == 0 {
		return engine.Config{}, errors.New("test-facing is empty: omit it to declare no test-facing packages")
	}
	if s.Roots != nil && len(s.Roots) == 0 {
		return engine.Config{}, errors.New("roots is empty: omit it to declare no root programs")
	}

	tests := true
	if s.Tests != nil {
		tests = *s.Tests
	}
	if len(s.TestFacing) > 0 && !tests {
		return engine.Config{}, errors.New("test-facing requires tests: the masked view is all it speaks to")
	}
	if s.APITestConsumers && len(s.API) == 0 {
		return engine.Config{}, errors.New("api-test-consumers requires api")
	}
	if s.APITestConsumers && !tests {
		return engine.Config{}, errors.New("api-test-consumers requires tests: the masked view is all it speaks to")
	}

	return engine.Config{
		Patterns:         s.Patterns,
		BuildTags:        s.BuildTags,
		Tests:            tests,
		API:              s.API,
		APIExempt:        exempts,
		APIGenerics:      generics,
		APITestConsumers: s.APITestConsumers,
		VocabularyNames:  s.VocabularyNames,
		TestFacing:       s.TestFacing,
		Roots:            s.Roots,
	}, nil
}

// exemptions validates api-exempt and supplies the default. An absent key
// takes the default and an explicitly empty list exempts nothing. Either list
// without api patterns is rejected rather than ignored: there is no surface
// for it to speak to, so the configuration does not mean what it says.
func exemptions(s Settings) ([]engine.Kind, error) {
	if len(s.API) == 0 {
		if s.APIExempt != nil {
			return nil, errors.New("api-exempt requires api")
		}
		return nil, nil
	}
	if s.APIExempt == nil {
		return []engine.Kind{engine.KindMethod}, nil
	}

	exempts := make([]engine.Kind, 0, len(s.APIExempt))
	for _, name := range s.APIExempt {
		kind := engine.Kind(name)
		if !slices.Contains(engine.Kinds(), kind) {
			return nil, fmt.Errorf("unknown api-exempt %q: want one of %s", name, kindNames())
		}
		exempts = append(exempts, kind)
	}
	return exempts, nil
}

// genericRooting validates api-generics, leaving the default to the engine so
// that every driver reads an absent key the same way. Like api-exempt it says
// nothing without api patterns, so declaring it alone is an error rather than
// a setting that quietly roots nothing.
func genericRooting(s Settings) (engine.GenericRooting, error) {
	if s.APIGenerics == nil {
		return "", nil
	}
	if len(s.API) == 0 {
		return "", errors.New("api-generics requires api")
	}
	rooting := engine.GenericRooting(*s.APIGenerics)
	if !slices.Contains(engine.GenericRootings(), rooting) {
		return "", fmt.Errorf("unknown api-generics %q: want one of %s", *s.APIGenerics, rootingNames())
	}
	return rooting, nil
}

func rootingNames() string {
	names := make([]string, 0, len(engine.GenericRootings()))
	for _, rooting := range engine.GenericRootings() {
		names = append(names, string(rooting))
	}
	return strings.Join(names, ", ")
}

func kindNames() string {
	names := make([]string, 0, len(engine.Kinds()))
	for _, kind := range engine.Kinds() {
		names = append(names, string(kind))
	}
	return strings.Join(names, ", ")
}

// moduleRoot walks up from the working directory to the nearest go.mod, so
// the analysis covers the whole module however deep golangci-lint was invoked.
// A subdirectory would otherwise miss the module's main packages and report
// everything they alone reach as dead.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod at or above %s", dir)
		}
		dir = parent
	}
}
