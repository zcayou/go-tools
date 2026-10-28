package zenum

import (
	"go/token"
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/internal/ruleset"
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

// Name is what golangci-lint knows this linter by: the key under
// linters.settings.custom, the entry in linters.enable, and the token
// a //nolint:zenum directive must carry.
const Name = "zenum"

// Settings is the decoded linters.settings.custom.zenum.settings block.
// Decoding rejects keys it does not know, so an unrecognized key fails the run.
//
// Each entry is a property of a type, applied wherever the type is used,
// rather than a one-off suppression; a one-off goes through a nolint directive
// or a linters.exclusions rule. An entry that suppresses nothing fails the run.
type Settings struct {
	// Selection chooses which rules run, through the default, enable and disable
	// keys. Every rule is standard.
	ruleset.Selection

	// OpenEnums are types whose values come from outside the module, such
	// as a wire ID declaring a few well-known constants. They are no enum to any
	// rule.
	OpenEnums []Exception `json:"open-enums"`

	// ZeroDefaults are enums whose zero member is the documented default.
	ZeroDefaults []Exception `json:"zero-defaults"`

	// RelatedEnums are pairs whose tokens overlap because they classify different
	// things.
	RelatedEnums []Relation `json:"related-enums"`

	// OrderedEnums are enums whose ordinals are an order, such as a severity
	// whose numbers rise with it.
	OrderedEnums []Exception `json:"ordered-enums"`

	// OrderedInstantiations has ordered-comparison judge a generic instantiated
	// with an enum whose type parameter's constraint orders it, as slices.Sort's
	// does. Defaults to false: a sort decides no membership, unless the module
	// holds output ordered by ordinal to follow declaration order.
	OrderedInstantiations *bool `json:"ordered-instantiations"`

	// Tests has the use-site rules judge test files. Defaults to true, matching
	// golangci-lint's run.tests, which a plugin cannot read.
	Tests *bool `json:"tests"`

	// BuildTags are extra build tags. golangci-lint's run.build-tags cannot
	// be read from a plugin, so it has to be repeated here.
	BuildTags []string `json:"build-tags"`
}

// Exception names one type, as import/path.Name, and why it is excepted.
type Exception struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// Relation names two types, as import/path.Name, and why their tokens may
// overlap. The pair is unordered.
type Relation struct {
	First  string `json:"first"`
	Second string `json:"second"`
	Reason string `json:"reason"`
}

// Plugin adapts the enum engine to golangci-lint's per-package linter
// contract. It holds the single analysis the whole run shares.
type Plugin struct {
	config    engine.Config
	buildTags []string
	linter    *wholemodule.Linter
}

var _ register.LinterPlugin = (*Plugin)(nil)

// New builds the plugin from the raw settings golangci-lint decoded out
// of the configuration file. It satisfies [register.NewPlugin].
//
// It decodes and validates, and touches nothing else. golangci-lint constructs
// every configured plugin whether or not the run enables it, and a failure here
// aborts configuration loading, so whether an entry names an enum waits
// until the analysis actually runs. Every error is a [*ConfigError].
func New(settings any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](settings)
	if err != nil {
		return nil, &ConfigError{Problem: ProblemUndecodableSettings, cause: err}
	}
	config, err := configFrom(s)
	if err != nil {
		return nil, err
	}
	p := &Plugin{config: config, buildTags: s.BuildTags}
	p.linter = wholemodule.New(Name, analyzerDoc, p.analyze)
	return p, nil
}

// BuildAnalyzers returns the analyzer this linter runs, alongside the one
// that keeps its results from being cached.
func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return p.linter.Analyzers(), nil
}

// GetLoadMode asks for syntax only. The analyzer reads nothing but file names
// and positions from the pass — the engine reads its own typed load — so making
// golangci-lint type check on this linter's behalf would be wasted work.
func (p *Plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}

// configFrom validates every entry: a reason that says why, and a type named
// the way the engine looks one up.
func configFrom(s Settings) (engine.Config, error) {
	rules, err := catalog.Resolve(s.Selection)
	if err != nil {
		return engine.Config{}, &ConfigError{Problem: ProblemRuleSelection, cause: err}
	}
	// A setting whose every rule is off configures nothing.
	for _, given := range []struct {
		setting engine.Setting
		set     bool
	}{
		{engine.SettingZeroDefaults, s.ZeroDefaults != nil},
		{engine.SettingRelatedEnums, s.RelatedEnums != nil},
		{engine.SettingOrderedEnums, s.OrderedEnums != nil},
		{engine.SettingTests, s.Tests != nil},
		{engine.SettingOrderedInstantiations, s.OrderedInstantiations != nil},
	} {
		if given.set && !rules.Any(serves(given.setting)...) {
			return engine.Config{}, &ConfigError{Problem: ProblemServesNoRule, Setting: given.setting}
		}
	}

	config := engine.Config{
		Tests:                 s.Tests == nil || *s.Tests,
		OrderedInstantiations: s.OrderedInstantiations != nil && *s.OrderedInstantiations,
		Disabled:              rules.Disabled(catalog),
	}
	for i, entry := range s.OpenEnums {
		if err := validate(engine.SettingOpenEnums, i, entry.Reason, entry.Type); err != nil {
			return engine.Config{}, err
		}
		config.OpenEnums = append(config.OpenEnums, entry.Type)
	}
	for i, entry := range s.ZeroDefaults {
		if err := validate(engine.SettingZeroDefaults, i, entry.Reason, entry.Type); err != nil {
			return engine.Config{}, err
		}
		config.ZeroDefaults = append(config.ZeroDefaults, entry.Type)
	}
	for i, entry := range s.RelatedEnums {
		if err := validate(engine.SettingRelatedEnums, i, entry.Reason, entry.First, entry.Second); err != nil {
			return engine.Config{}, err
		}
		config.RelatedEnums = append(config.RelatedEnums,
			engine.Pair{First: entry.First, Second: entry.Second})
	}
	for i, entry := range s.OrderedEnums {
		if err := validate(engine.SettingOrderedEnums, i, entry.Reason, entry.Type); err != nil {
			return engine.Config{}, err
		}
		config.OrderedEnums = append(config.OrderedEnums, entry.Type)
	}
	return config, nil
}

func validate(setting engine.Setting, index int, reason string, types ...string) error {
	if strings.TrimSpace(reason) == "" {
		return &ConfigError{Problem: ProblemUnexplainedException, Setting: setting, Index: index}
	}
	for _, typ := range types {
		if !qualified(typ) {
			return &ConfigError{Problem: ProblemUnqualifiedType, Setting: setting, Index: index, Type: typ}
		}
	}
	return nil
}

// qualified reports whether text names a type as import/path.Name: a non-empty
// path, a dot, and an identifier. The name follows the last dot, since
// an import path's elements may hold dots of their own and an identifier
// cannot.
func qualified(text string) bool {
	path, name, ok := strings.CutLast(text, ".")
	return ok && path != "" && token.IsIdentifier(name)
}

// catalog is the rules a selection chooses from.
var catalog = ruleset.Catalog[engine.Rule]{Rules: []engine.Rule{
	engine.RuleStringEnum,
	engine.RuleUnnamed,
	engine.RuleUnreadableName,
	engine.RuleAmbiguousName,
	engine.RuleZeroMember,
	engine.RuleRestated,
	engine.RuleTokenDecision,
	engine.RuleForgedConversion,
	engine.RuleNumericVerb,
	engine.RuleStringBypass,
	engine.RuleLaunderedConversion,
	engine.RuleOrderedComparison,
	engine.RuleMemberBound,
}}

// useSite are the rules judging how code uses an enum, which judge test files
// as they judge the code beside them.
var useSite = []engine.Rule{
	engine.RuleTokenDecision,
	engine.RuleForgedConversion,
	engine.RuleLaunderedConversion,
	engine.RuleNumericVerb,
	engine.RuleStringBypass,
	engine.RuleOrderedComparison,
	engine.RuleMemberBound,
}

// serves returns the rules a setting configures. open-enums takes a type out
// of every rule, so it configures whichever run.
func serves(setting engine.Setting) []engine.Rule {
	switch setting {
	case engine.SettingZeroDefaults:
		return []engine.Rule{engine.RuleZeroMember}
	case engine.SettingRelatedEnums:
		return []engine.Rule{engine.RuleRestated}
	case engine.SettingOrderedEnums:
		return []engine.Rule{engine.RuleOrderedComparison, engine.RuleMemberBound}
	case engine.SettingTests:
		return useSite
	case engine.SettingOrderedInstantiations:
		return []engine.Rule{engine.RuleOrderedComparison}
	default:
		return catalog.Rules
	}
}
