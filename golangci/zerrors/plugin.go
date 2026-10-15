package zerrors

import (
	"go/token"
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/internal/ruleset"
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

// Name is what golangci-lint knows this linter by: the key under
// linters.settings.custom, the entry in linters.enable, and the token
// a //nolint:zerrors directive must carry.
const Name = "zerrors"

// Settings is the decoded linters.settings.custom.zerrors.settings block.
// Decoding rejects keys it does not know, so an unrecognized key fails the run.
type Settings struct {
	// Selection chooses which rules run, through the default, enable and disable
	// keys. Every rule is standard.
	ruleset.Selection

	// API are package patterns, resolved against the module root, narrowing
	// the API packages: those whose exported functions and types are judged,
	// and whose exported error types and sentinels a caller can name. Omit the key
	// for ./...; an explicitly empty list is rejected. A main package or one under
	// an internal element is never API, whatever pattern matches it.
	API []string `json:"api"`

	// Trusted are import paths, each optionally ending in /..., whose errors
	// count as structured, and whose interfaces a module type converted to one
	// answers to: its methods implementing it are judged.
	Trusted []string `json:"trusted"`

	// Protocols are interface methods whose implementations unstructured-return
	// does not judge, because the interface's consumer classifies whatever they
	// return.
	Protocols []Protocol `json:"protocols"`

	// Sentinels counts an exported package-level error variable of an API
	// package as structured. Defaults to true; with it off, the variable counts
	// as what initializes it.
	Sentinels *bool `json:"sentinels"`

	// Base is the import/path.Name of a struct every error type embeds by value,
	// or of an interface every error type implements. Setting it turns
	// missing-base on, and makes an error type carrying it nameable.
	Base *string `json:"base"`

	// BasePackages are package patterns, resolved against the module root,
	// naming the packages whose error types missing-base judges: the part
	// of the module following the idiom the base belongs to. Omit the key
	// for ./...; an explicitly empty list, or one set without base, is rejected.
	// Unlike api, a pattern may match main and internal packages.
	BasePackages []string `json:"base-packages"`

	// BuildTags are extra build tags. golangci-lint's run.build-tags cannot
	// be read from a plugin, so it has to be repeated here.
	BuildTags []string `json:"build-tags"`
}

// Protocol is one interface method whose implementations are not judged.
type Protocol struct {
	// Method is the interface method, written import/path.Interface.Method.
	Method string `json:"method"`

	// Reason says why the interface's consumer wants raw errors.
	Reason string `json:"reason"`
}

// Plugin adapts the whole-module engine to golangci-lint's per-package linter
// contract. It holds the single analysis the whole run shares.
type Plugin struct {
	config engine.Config
	linter *wholemodule.Linter
}

var _ register.LinterPlugin = (*Plugin)(nil)

// New builds the plugin from the raw settings golangci-lint decoded out
// of the configuration file. It satisfies [register.NewPlugin], and fails
// with an [*engine.ConfigError].
//
// It decodes and validates, and touches nothing else. golangci-lint constructs
// every configured plugin whether or not the run enables it, and a failure here
// aborts configuration loading, so whatever needs the module to check waits
// for the analysis.
func New(settings any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](settings)
	if err != nil {
		return nil, engine.Undecodable(err)
	}
	config, err := configFrom(s)
	if err != nil {
		return nil, err
	}
	p := &Plugin{config: config}
	p.linter = wholemodule.New(Name, analyzerDoc, p.analyze)
	return p, nil
}

// BuildAnalyzers returns the analyzer this linter runs, alongside the one
// that keeps its results from being cached.
func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return p.linter.Analyzers(), nil
}

// GetLoadMode asks for syntax only. The analyzer reads nothing but file names
// and positions from the pass — the engine shares a typed load of its own — so
// making golangci-lint type check on this linter's behalf would be wasted
// work.
func (p *Plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}

// configFrom validates the settings no module could satisfy, and supplies
// the defaults.
func configFrom(s Settings) (engine.Config, error) {
	rules, err := catalog.Resolve(s.Selection)
	if err != nil {
		return engine.Config{}, engine.RuleSelection(err)
	}
	if err := s.serving(rules); err != nil {
		return engine.Config{}, err
	}
	if s.API != nil && len(s.API) == 0 {
		return engine.Config{}, &engine.ConfigError{Setting: engine.SettingAPI, Problem: engine.ProblemEmpty}
	}
	for _, entry := range s.Trusted {
		if path, _ := strings.CutSuffix(entry, "/..."); !importPath(path) {
			return engine.Config{}, &engine.ConfigError{
				Setting: engine.SettingTrusted, Problem: engine.ProblemMalformed, Value: entry,
			}
		}
	}
	methods := make([]string, 0, len(s.Protocols))
	for _, protocol := range s.Protocols {
		qualified, method, ok := strings.CutLast(protocol.Method, ".")
		if !ok || !qualifiedName(qualified) || !token.IsIdentifier(method) {
			return engine.Config{}, &engine.ConfigError{
				Setting: engine.SettingProtocols, Problem: engine.ProblemMalformed, Value: protocol.Method,
			}
		}
		if strings.TrimSpace(protocol.Reason) == "" {
			return engine.Config{}, &engine.ConfigError{
				Setting: engine.SettingProtocols, Problem: engine.ProblemNoReason, Value: protocol.Method,
			}
		}
		methods = append(methods, protocol.Method)
	}
	base := ""
	if s.Base != nil {
		if !qualifiedName(*s.Base) {
			return engine.Config{}, &engine.ConfigError{
				Setting: engine.SettingBase, Problem: engine.ProblemMalformed, Value: *s.Base,
			}
		}
		base = *s.Base
	}
	if s.BasePackages != nil {
		switch {
		case len(s.BasePackages) == 0:
			return engine.Config{}, &engine.ConfigError{Setting: engine.SettingBasePackages, Problem: engine.ProblemEmpty}
		case s.Base == nil:
			return engine.Config{}, &engine.ConfigError{
				Setting: engine.SettingBasePackages, Problem: engine.ProblemWithoutBase,
			}
		}
	}
	sentinels := true
	if s.Sentinels != nil {
		sentinels = *s.Sentinels
	}

	return engine.Config{
		API:          s.API,
		Trusted:      s.Trusted,
		Protocols:    methods,
		Sentinels:    sentinels,
		Base:         base,
		BasePackages: s.BasePackages,
		BuildTags:    s.BuildTags,
		Disabled:     rules.Disabled(catalog),
	}, nil
}

// serving rejects a setting given while every rule it configures is off,
// since it then configures nothing.
func (s Settings) serving(rules ruleset.Enabled[engine.Rule]) error {
	for _, given := range []struct {
		setting engine.Setting
		set     bool
	}{
		{engine.SettingAPI, s.API != nil},
		{engine.SettingTrusted, s.Trusted != nil},
		{engine.SettingProtocols, s.Protocols != nil},
		{engine.SettingSentinels, s.Sentinels != nil},
		{engine.SettingBase, s.Base != nil},
		{engine.SettingBasePackages, s.BasePackages != nil},
	} {
		if given.set && !rules.Any(given.setting.Serves()...) {
			return &engine.ConfigError{Setting: given.setting, Problem: engine.ProblemServesNoRule}
		}
	}
	return nil
}

// catalog is the rules a selection chooses from.
var catalog = ruleset.Catalog[engine.Rule]{Rules: []engine.Rule{
	engine.RuleUnstructuredReturn,
	engine.RuleTextDecision,
	engine.RuleLiteralField,
	engine.RuleRenderedField,
	engine.RuleMissingBase,
}}

// qualifiedName reports whether text is written import/path.Name.
func qualifiedName(text string) bool {
	path, name, ok := strings.CutLast(text, ".")
	return ok && importPath(path) && token.IsIdentifier(name)
}

// importPath reports whether text reads as an import path: slash-separated
// elements, none empty, relative or a wildcard.
func importPath(text string) bool {
	if text == "" || strings.ContainsAny(text, " \t\\") {
		return false
	}
	for element := range strings.SplitSeq(text, "/") {
		if element == "" || element == "." || element == ".." || strings.Contains(element, "...") {
			return false
		}
	}
	return true
}
