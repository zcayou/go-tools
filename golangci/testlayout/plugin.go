package testlayout

import (
	"errors"
	"fmt"
	"slices"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

// Name is what golangci-lint knows this linter by: the key under
// linters.settings.custom, the entry in linters.enable, and the token
// a //nolint:testlayout directive must carry.
const Name = "testlayout"

// Settings is the decoded linters.settings.custom.testlayout.settings block.
//
// Decoding rejects unknown fields, so a misspelled key fails the run rather
// than being silently ignored. The zero value is the default configuration:
// black-box test files only, each named for the source it exercises.
type Settings struct {
	// Whitebox configures test files that declare the package under test and so
	// can reach its unexported identifiers. Absent means not allowed.
	Whitebox *Category `json:"whitebox"`

	// Blackbox configures test files that declare the external <pkg>_test package
	// and so see only what a consumer sees. Absent means allowed.
	Blackbox *Category `json:"blackbox"`

	// AdapterPatterns are the names a file holding the Ginkgo adapter — the go
	// test entry point calling RunSpecs — may have. Absent means
	// [DefaultAdapterPatterns].
	AdapterPatterns []string `json:"adapter-patterns"`
}

// Category is one kind of test file: whether files of that kind may exist,
// and what they may be named.
type Category struct {
	// Allowed reports whether a test file of this kind may exist at all.
	Allowed *bool `json:"allowed"`

	// Patterns are the names a spec-bearing file of this kind may have. Absent
	// means [DefaultPatterns]; an explicit empty list allows none, which leaves
	// HelperPatterns as the only names this kind of file may have.
	Patterns []string `json:"patterns"`

	// HelperPatterns are the names a file of this kind carrying no specs may have.
	// A file matching one is required to carry no specs, so a name saying
	// "helpers" cannot quietly hold the specs for a source file. Absent means
	// [DefaultHelperPatterns]; an explicit empty list allows no such file.
	HelperPatterns []string `json:"helper-patterns"`
}

// DefaultPatterns are the names a spec-bearing test file may have when
// a category does not name its own: one named for a source file beside it,
// and the suite file, which is named for the suite rather than for any source.
func DefaultPatterns() []string {
	return []string{sourceToken + testSuffix, "suite_test.go"}
}

// DefaultHelperPatterns are the names a test file carrying no specs may have
// when a category does not name its own.
func DefaultHelperPatterns() []string {
	return []string{"helpers_test.go", "fakes_test.go"}
}

// DefaultAdapterPatterns are the names a file holding the Ginkgo adapter may
// have when settings do not name their own.
func DefaultAdapterPatterns() []string {
	return []string{"suite_test.go"}
}

// Plugin adapts the analyzer to golangci-lint's module plugin contract.
type Plugin struct {
	analyzer *analysis.Analyzer
}

var _ register.LinterPlugin = (*Plugin)(nil)

// New builds the plugin from the raw settings golangci-lint decoded out
// of the configuration file. It satisfies [register.NewPlugin].
func New(settings any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](settings)
	if err != nil {
		return nil, fmt.Errorf("%s: decoding settings: %w", Name, err)
	}
	analyzer, err := NewAnalyzer(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", Name, err)
	}

	return &Plugin{analyzer: analyzer}, nil
}

// BuildAnalyzers returns the single analyzer this linter runs.
func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{p.analyzer}, nil
}

// GetLoadMode reports that the rules are syntactic, sparing consumers the cost
// of type checking for this linter.
func (p *Plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}

// config is [Settings] with the defaults filled in and every pattern compiled.
type config struct {
	whitebox category
	blackbox category
	adapters patternSet
}

// category is one validated [Category]. name is the settings key it came from,
// so a diagnostic points at the knob that decides it.
type category struct {
	name    string
	allowed bool
	specs   patternSet
	helpers patternSet
}

func (c config) category(blackbox bool) category {
	if blackbox {
		return c.blackbox
	}
	return c.whitebox
}

// names lists every name a file of this kind may have.
func (c category) names() string {
	return slices.Concat(c.specs, c.helpers).String()
}

// specsBelong says where the specs a helper file holds should have gone.
// An explicitly empty list of spec patterns leaves nowhere to send them, so
// the setting that decided it is named instead.
func (c category) specsBelong() string {
	if len(c.specs) == 0 {
		return c.name + ".patterns is empty, so no file of this kind may carry specs"
	}
	return "move them to a file named " + c.specs.String()
}

// helpersBelong says what a file named for a source but carrying no specs
// should have been called. An explicitly empty list of helper patterns leaves
// no such name, so the setting that decided it is named instead.
func (c category) helpersBelong() string {
	if len(c.helpers) == 0 {
		return c.name + ".helper-patterns is empty, so every file of this kind must carry specs"
	}
	return "a file carrying only helpers must be named " + c.helpers.String()
}

// resolve fills in the defaults and compiles the patterns, rejecting
// a configuration no file could satisfy.
func (s Settings) resolve() (config, error) {
	whitebox, err := s.Whitebox.resolve("whitebox", false)
	if err != nil {
		return config{}, err
	}
	blackbox, err := s.Blackbox.resolve("blackbox", true)
	if err != nil {
		return config{}, err
	}
	if !whitebox.allowed && !blackbox.allowed {
		return config{}, errors.New("neither whitebox nor blackbox is allowed, so no test file could exist")
	}

	adapters, err := patternsOr(s.AdapterPatterns, DefaultAdapterPatterns())
	if err != nil {
		return config{}, fmt.Errorf("adapter-patterns: %w", err)
	}
	if len(adapters) == 0 {
		return config{}, errors.New("adapter-patterns is empty, so no file could hold the Ginkgo adapter")
	}

	return config{whitebox: whitebox, blackbox: blackbox, adapters: adapters}, nil
}

// resolve reads a category block, taking the defaults when the block is absent
// altogether.
func (c *Category) resolve(name string, allowed bool) (category, error) {
	if c == nil {
		c = new(Category)
	}

	out := category{name: name, allowed: allowed}
	if c.Allowed != nil {
		out.allowed = *c.Allowed
	}

	var err error
	if out.specs, err = patternsOr(c.Patterns, DefaultPatterns()); err != nil {
		return category{}, fmt.Errorf("%s.patterns: %w", name, err)
	}
	if out.helpers, err = patternsOr(c.HelperPatterns, DefaultHelperPatterns()); err != nil {
		return category{}, fmt.Errorf("%s.helper-patterns: %w", name, err)
	}
	if out.allowed && len(out.specs)+len(out.helpers) == 0 {
		return category{}, fmt.Errorf("%s is allowed but no pattern permits any name", name)
	}
	return out, nil
}

// patternsOr compiles texts, or the fallback when the setting was absent.
// An explicitly empty list stays empty: "no file of this sort is allowed"
// is a thing a repository may mean, while leaving the key out is not.
func patternsOr(texts, fallback []string) (patternSet, error) {
	if texts == nil {
		texts = fallback
	}
	return newPatternSet(texts)
}
