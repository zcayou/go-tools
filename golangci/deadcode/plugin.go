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
	// and test-only references count as uses. Defaults to true, matching
	// golangci-lint's run.tests, which this cannot read.
	Tests *bool `json:"tests"`

	// API are package patterns whose exported surface consumers reach.
	API []string `json:"api"`

	// APIExempt is the declaration kinds an API package exempts: func, method,
	// type, const, var, or interface-method. Defaults to method when API is set.
	// Requires API.
	APIExempt []string `json:"api-exempt"`
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
	if s.Patterns != nil && len(s.Patterns) == 0 {
		return engine.Config{}, errors.New("patterns is empty: omit it to analyze the whole module")
	}

	tests := true
	if s.Tests != nil {
		tests = *s.Tests
	}

	return engine.Config{
		Patterns:  s.Patterns,
		BuildTags: s.BuildTags,
		Tests:     tests,
		API:       s.API,
		APIExempt: exempts,
	}, nil
}

// exemptions validates api-exempt and supplies the default. Exemptions without
// api patterns are rejected rather than ignored: nothing would be exempt, so
// the configuration does not mean what it says.
func exemptions(s Settings) ([]engine.Kind, error) {
	if len(s.API) == 0 {
		if len(s.APIExempt) > 0 {
			return nil, errors.New("api-exempt requires api")
		}
		return nil, nil
	}
	if len(s.APIExempt) == 0 {
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
