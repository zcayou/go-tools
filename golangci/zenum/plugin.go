package zenum

import (
	"go/token"
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

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
	// OpenEnums are types whose values come from outside the module, such
	// as a wire ID declaring a few well-known constants. They are no enum to any
	// rule.
	OpenEnums []Exception `json:"open-enums"`

	// ZeroDefaults are enums whose zero member is the documented default.
	ZeroDefaults []Exception `json:"zero-defaults"`

	// RelatedEnums are pairs whose tokens overlap because they classify different
	// things.
	RelatedEnums []Relation `json:"related-enums"`

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
	var config engine.Config
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
