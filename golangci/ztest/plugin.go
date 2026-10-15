package ztest

import (
	"fmt"
	"slices"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/internal/ruleset"
	"github.com/zcayou/go-tools/golangci/ztest/internal/filepattern"
	"github.com/zcayou/go-tools/golangci/ztest/layout"
	"github.com/zcayou/go-tools/golangci/ztest/spec"
)

// Name is what golangci-lint knows this linter by: the key under
// linters.settings.custom, the entry in linters.enable, and the token
// a //nolint:ztest directive must carry.
const Name = "ztest"

// testSuffix is what every test file's name ends in.
const testSuffix = "_test.go"

// Settings is the decoded linters.settings.custom.ztest.settings block.
//
// Decoding rejects unknown fields, so a misspelled key fails the run rather
// than being silently ignored. The zero value is the default configuration:
// every rule runs; beside a source, black-box test files only, each named
// for the source it exercises; in a directory holding no source, standalone
// files named for what they exercise; every API package's specs owe
// an identified error; and a test file named for a source pins the renderings
// that source declares.
type Settings struct {
	// Selection chooses which rules run, through the default, enable and disable
	// keys. Every rule is standard.
	ruleset.Selection

	// Whitebox configures test files that declare the package under test and so
	// can reach its unexported identifiers. Absent means not allowed.
	Whitebox *Category `json:"whitebox"`

	// Blackbox configures test files that declare the external <pkg>_test package
	// and so see only what a consumer sees. Absent means allowed.
	Blackbox *Category `json:"blackbox"`

	// Standalone configures test files in a directory holding no source to test,
	// where neither of the other kinds applies. Absent means allowed.
	Standalone *Category `json:"standalone"`

	// AdapterPatterns are the names a file holding the Ginkgo adapter — the go
	// test entry point calling RunSpecs — may have. Absent means
	// [DefaultAdapterPatterns].
	AdapterPatterns []string `json:"adapter-patterns"`

	// API are package patterns, resolved against the module root, narrowing
	// the packages whose specs unidentified-error judges. Absent means ./...;
	// an explicit empty list is rejected.
	API []string `json:"api"`

	// RenderingPatterns are the names of the test files that pin the rendering
	// of the source they are named for, each holding <source> once. Absent
	// means [DefaultRenderingPatterns]; an explicit empty list means no file
	// pins a rendering.
	RenderingPatterns []string `json:"rendering-patterns"`

	// BuildTags are extra build tags, under which the api patterns resolve.
	// golangci-lint's run.build-tags cannot be read from a plugin, so it has
	// to be repeated here.
	BuildTags []string `json:"build-tags"`
}

// Category is one kind of test file: whether files of that kind may exist,
// and what they may be named.
type Category struct {
	// Allowed reports whether a test file of this kind may exist at all.
	Allowed *bool `json:"allowed"`

	// Patterns are the names a spec-bearing file of this kind may have. Absent
	// means the kind's default, [DefaultPatterns] or [DefaultStandalonePatterns];
	// an explicit empty list allows none, which leaves HelperPatterns as the only
	// names this kind of file may have.
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
	return []string{filepattern.SourceToken + testSuffix, "suite_test.go"}
}

// DefaultStandalonePatterns are the names a spec-bearing test file may have
// in a directory holding no source, when the standalone category does not name
// its own: any test file, since there is no source to be named for.
func DefaultStandalonePatterns() []string {
	return []string{"*" + testSuffix}
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

// DefaultRenderingPatterns are the names of the test files that pin a rendering
// when settings do not name their own: the spec named for the source
// it exercises.
func DefaultRenderingPatterns() []string {
	return []string{filepattern.SourceToken + testSuffix}
}

// Plugin adapts the analyzer to golangci-lint's module plugin contract.
type Plugin struct {
	analyzer *analysis.Analyzer
}

var _ register.LinterPlugin = (*Plugin)(nil)

// New builds the plugin from the raw settings golangci-lint decoded out
// of the configuration file. It satisfies [register.NewPlugin]. It reads
// nothing from disk: an explicit api setting is resolved by the run, which
// is where a pattern matching nothing fails. Every error is a [*ConfigError].
func New(settings any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](settings)
	if err != nil {
		return nil, &ConfigError{Problem: ProblemUndecodable, cause: err}
	}
	analyzer, err := NewAnalyzer(s)
	if err != nil {
		return nil, err
	}
	return &Plugin{analyzer: analyzer}, nil
}

// BuildAnalyzers returns the single analyzer this linter runs.
func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{p.analyzer}, nil
}

// GetLoadMode reports that the spec rules need type information: every name
// resolves through it, and an enum is known by the fact its declaring
// package's pass exports. The layout rules read syntax, and files beyond
// the build from disk, so they ask nothing more of it.
func (p *Plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

// Config is [Settings] resolved: what each half of the linter judges under.
type Config struct {
	// Layout configures the rules about where tests live and what they are named.
	Layout layout.Config

	// Spec configures the rules about what specs assert and the context they
	// run under.
	Spec spec.Config
}

// Resolve fills in the defaults and compiles the patterns, rejecting
// with a [*ConfigError] a configuration no run could satisfy. Nothing here
// needs the module, so what only the module can answer waits for the run.
func Resolve(s Settings) (Config, error) {
	rules, err := catalog.Resolve(s.Selection)
	if err != nil {
		return Config{}, &ConfigError{Problem: ProblemRuleSelection, cause: err}
	}
	if err = s.serving(rules); err != nil {
		return Config{}, err
	}

	whitebox, err := s.Whitebox.resolve(layout.Category{Kind: layout.KindWhitebox}, DefaultPatterns())
	if err != nil {
		return Config{}, err
	}
	blackbox, err := s.Blackbox.resolve(layout.Category{Kind: layout.KindBlackbox, Allowed: true}, DefaultPatterns())
	if err != nil {
		return Config{}, err
	}
	standalone, err := s.Standalone.resolve(
		layout.Category{Kind: layout.KindStandalone, Allowed: true, Sourceless: true}, DefaultStandalonePatterns())
	if err != nil {
		return Config{}, err
	}
	if !whitebox.Allowed && !blackbox.Allowed && !standalone.Allowed {
		return Config{}, &ConfigError{Problem: ProblemNoKindAllowed}
	}
	adapters, err := patternsOr(s.AdapterPatterns, DefaultAdapterPatterns())
	if err != nil {
		return Config{}, &ConfigError{Setting: SettingAdapterPatterns, Problem: ProblemMalformedPattern, cause: err}
	}
	if len(adapters) == 0 {
		return Config{}, &ConfigError{Setting: SettingAdapterPatterns, Problem: ProblemNoAdapterName}
	}

	if s.API != nil && len(s.API) == 0 {
		return Config{}, &ConfigError{Setting: SettingAPI, Problem: ProblemEmptyAPI}
	}
	renderings, err := patternsOr(s.RenderingPatterns, DefaultRenderingPatterns())
	if err != nil {
		return Config{}, &ConfigError{Setting: SettingRenderingPatterns, Problem: ProblemMalformedPattern, cause: err}
	}
	for _, p := range renderings {
		if !p.NamesSource() {
			return Config{}, &ConfigError{
				Setting: SettingRenderingPatterns, Problem: ProblemPatternNamesNoSource, Pattern: p.String(),
			}
		}
	}

	config := Config{
		Layout: layout.Config{
			Whitebox: whitebox, Blackbox: blackbox, Standalone: standalone, Adapters: adapters,
		},
		Spec: spec.Config{Renderings: renderings},
	}
	if s.API != nil {
		config.Spec.API = &apiScope{patterns: s.API, buildTags: s.BuildTags}
	}
	for _, rule := range rules.Disabled(catalog) {
		switch rule := rule.(type) {
		case layout.Rule:
			config.Layout.Disabled = append(config.Layout.Disabled, rule)
		case spec.Rule:
			config.Spec.Disabled = append(config.Spec.Disabled, rule)
		}
	}
	return config, nil
}

// catalog is the rules a selection chooses from, both halves' together.
var catalog = ruleset.Catalog[fmt.Stringer]{Rules: slices.Concat(layoutRules, specRules)}

// layoutRules and specRules are each half's rules.
var (
	layoutRules = []fmt.Stringer{
		layout.RuleTestPackage,
		layout.RuleTestFileName,
		layout.RuleSpecLessTestFile,
		layout.RuleSpecsInHelperFile,
		layout.RuleGinkgoAdapterFile,
		layout.RuleGinkgoAdapterMissing,
		layout.RuleGinkgoAdapterDuplicate,
		layout.RuleSuiteFileContents,
		layout.RuleMisplacedFixture,
		layout.RuleSingleConsumerHelperFile,
	}
	specRules = []fmt.Stringer{
		spec.RuleUnidentifiedError,
		spec.RuleTextAssertion,
		spec.RuleTokenAssertion,
		spec.RuleBackgroundContext,
	}
)

// serving rejects a setting given while every rule it configures is off,
// since it then configures nothing.
func (s Settings) serving(rules ruleset.Enabled[fmt.Stringer]) error {
	for _, given := range []struct {
		kind    layout.Kind
		setting Setting
		set     bool
	}{
		{layout.KindWhitebox, 0, s.Whitebox != nil},
		{layout.KindBlackbox, 0, s.Blackbox != nil},
		{layout.KindStandalone, 0, s.Standalone != nil},
		{0, SettingAdapterPatterns, s.AdapterPatterns != nil},
		{0, SettingAPI, s.API != nil},
		{0, SettingBuildTags, s.BuildTags != nil},
		{0, SettingRenderingPatterns, s.RenderingPatterns != nil},
	} {
		e := &ConfigError{Kind: given.kind, Setting: given.setting, Problem: ProblemServesNoRule}
		if given.set && !rules.Any(e.serves()...) {
			return e
		}
	}
	return nil
}

// serves returns the rules the setting at fault configures. Every layout
// setting describes the layout each layout rule judges.
func (e *ConfigError) serves() []fmt.Stringer {
	if e.Kind != 0 {
		return layoutRules
	}
	switch e.Setting {
	case SettingPatterns, SettingHelperPatterns, SettingAdapterPatterns:
		return layoutRules
	case SettingAPI, SettingBuildTags:
		return []fmt.Stringer{spec.RuleUnidentifiedError}
	case SettingRenderingPatterns:
		return []fmt.Stringer{spec.RuleTextAssertion, spec.RuleTokenAssertion}
	default:
		return nil
	}
}

// resolve reads a category block over what the kind is before the block
// is read: its settings key, whether it is allowed when the key is absent,
// and whether its files sit beside a source. specs are the spec-bearing names
// it takes when none are configured.
func (c *Category) resolve(defaults layout.Category, specs []string) (layout.Category, error) {
	if c == nil {
		c = new(Category)
	}

	out := defaults
	if c.Allowed != nil {
		out.Allowed = *c.Allowed
	}

	var err error
	if out.Specs, err = patternsOr(c.Patterns, specs); err != nil {
		return layout.Category{}, &ConfigError{
			Kind: out.Kind, Setting: SettingPatterns, Problem: ProblemMalformedPattern, cause: err,
		}
	}
	if out.Helpers, err = patternsOr(c.HelperPatterns, DefaultHelperPatterns()); err != nil {
		return layout.Category{}, &ConfigError{
			Kind: out.Kind, Setting: SettingHelperPatterns, Problem: ProblemMalformedPattern, cause: err,
		}
	}
	if out.Allowed && len(out.Specs)+len(out.Helpers) == 0 {
		return layout.Category{}, &ConfigError{Kind: out.Kind, Problem: ProblemNoName}
	}
	if out.Sourceless {
		if err = namesNoSource(out.Specs, out.Kind, SettingPatterns); err != nil {
			return layout.Category{}, err
		}
		if err = namesNoSource(out.Helpers, out.Kind, SettingHelperPatterns); err != nil {
			return layout.Category{}, err
		}
	}
	return out, nil
}

// patternsOr compiles texts, or the fallback when the setting was absent.
// An explicitly empty list stays empty: "no file of this sort is allowed"
// is a thing a repository may mean, while leaving the key out is not.
func patternsOr(texts, fallback []string) (filepattern.Set, error) {
	if texts == nil {
		texts = fallback
	}
	return filepattern.NewSet(texts)
}

// namesNoSource rejects a pattern resolving [filepattern.SourceToken] in a set
// naming files that sit beside no source. kind and setting are where the set
// came from, for the error to point at.
func namesNoSource(set filepattern.Set, kind layout.Kind, setting Setting) error {
	for _, p := range set {
		if p.NamesSource() {
			return &ConfigError{Kind: kind, Setting: setting, Problem: ProblemPatternNamesSource, Pattern: p.String()}
		}
	}
	return nil
}
