package deadcode

import (
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
	"github.com/zcayou/go-tools/golangci/internal/ruleset"
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// Name is what golangci-lint knows this linter by: the key under
// linters.settings.custom, the entry in linters.enable, and the token
// a //nolint:deadcode directive must carry.
const Name = "deadcode"

// Settings is the decoded linters.settings.custom.deadcode.settings block.
// Decoding rejects keys it does not know, so an unrecognized key fails the run.
type Settings struct {
	// Selection chooses which rules beside the verdicts run, through the default,
	// enable and disable keys. The verdicts are the linter itself and are not
	// selectable. inert-field, the one such rule, is opt-in.
	ruleset.Selection

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

	// EnumNames credits each enum's published name. A named type whose own package
	// declares typed constants of it is an enum, and a String() string method
	// on a value receiver is its name: it counts as used, and what it calls stays
	// live, whether or not production ever renders a member. Other methods
	// of the type, and a String on a type with no declared constant, are judged
	// as ever. Off by default, because it states a convention rather than evidence
	// the program carries.
	EnumNames bool `json:"enum-names"`

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

	// EchoFunctions are functions whose reads of a field echo it into text
	// or an identity rather than act on it, so inert-field does not count them.
	// Each entry carries a reason. It serves inert-field.
	EchoFunctions []EchoFunction `json:"echo-functions"`
}

// EchoFunction names a function as import/path.Func, import/path.Type.Method
// or import/path.(*Type).Method, and why its reads do not act on a field.
type EchoFunction struct {
	Function string `json:"function"`
	Reason   string `json:"reason"`
}

// Plugin adapts the whole-program engine to golangci-lint's per-package linter
// contract. It holds the single analysis the whole run shares.
type Plugin struct {
	config engine.Config
	linter *wholemodule.Linter
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
// and positions from the pass — the engine does its own typed load — so making
// golangci-lint type check on this linter's behalf would be wasted work.
func (p *Plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}

// configFrom validates the settings.
func configFrom(s Settings) (engine.Config, error) {
	rules, err := rulesFrom(s)
	if err != nil {
		return engine.Config{}, err
	}
	exempts, err := exemptions(s)
	if err != nil {
		return engine.Config{}, err
	}
	generics, err := genericRooting(s)
	if err != nil {
		return engine.Config{}, err
	}
	for _, list := range []struct {
		setting engine.Setting
		values  []string
	}{
		{engine.SettingPatterns, s.Patterns},
		{engine.SettingTestFacing, s.TestFacing},
		{engine.SettingRoots, s.Roots},
	} {
		if list.values != nil && len(list.values) == 0 {
			return engine.Config{}, &engine.ConfigError{Setting: list.setting, Problem: engine.ProblemEmpty}
		}
	}

	tests := true
	if s.Tests != nil {
		tests = *s.Tests
	}
	if len(s.TestFacing) > 0 && !tests {
		return engine.Config{}, &engine.ConfigError{Setting: engine.SettingTestFacing, Problem: engine.ProblemRequiresTests}
	}
	if s.APITestConsumers && len(s.API) == 0 {
		return engine.Config{}, &engine.ConfigError{
			Setting: engine.SettingAPITestConsumers, Problem: engine.ProblemRequiresAPI,
		}
	}
	if s.APITestConsumers && !tests {
		return engine.Config{}, &engine.ConfigError{
			Setting: engine.SettingAPITestConsumers, Problem: engine.ProblemRequiresTests,
		}
	}

	var echoes []string
	for _, entry := range s.EchoFunctions {
		if strings.TrimSpace(entry.Function) == "" {
			return engine.Config{}, &engine.ConfigError{
				Setting: engine.SettingEchoFunctions, Problem: engine.ProblemNamesNoFunction,
			}
		}
		if strings.TrimSpace(entry.Reason) == "" {
			return engine.Config{}, &engine.ConfigError{
				Setting: engine.SettingEchoFunctions, Problem: engine.ProblemUnexplained, Value: entry.Function,
			}
		}
		echoes = append(echoes, entry.Function)
	}
	if s.EchoFunctions != nil && len(s.EchoFunctions) == 0 {
		return engine.Config{}, &engine.ConfigError{Setting: engine.SettingEchoFunctions, Problem: engine.ProblemEmpty}
	}

	return engine.Config{
		Patterns:         s.Patterns,
		BuildTags:        s.BuildTags,
		Tests:            tests,
		API:              s.API,
		APIExempt:        exempts,
		APIGenerics:      generics,
		APITestConsumers: s.APITestConsumers,
		EnumNames:        s.EnumNames,
		TestFacing:       s.TestFacing,
		Roots:            s.Roots,
		Rules:            rules,
		EchoFunctions:    echoes,
	}, nil
}

// catalog is the rules beside the verdicts a selection chooses from.
// A selection enabling none of them still leaves the verdicts.
var catalog = ruleset.Catalog[engine.Rule]{
	Rules:  []engine.Rule{engine.RuleInertField},
	OptIn:  []engine.Rule{engine.RuleInertField},
	Beside: true,
}

// rulesFrom resolves the rule selection, and rejects a rule enabled without
// what it requires and a setting serving no enabled rule.
func rulesFrom(s Settings) ([]engine.Rule, error) {
	enabled, err := catalog.Resolve(s.Selection)
	if err != nil {
		return nil, engine.RuleSelection(err)
	}
	if enabled.Has(engine.RuleInertField) && len(s.API) == 0 {
		return nil, &engine.ConfigError{Rule: engine.RuleInertField, Problem: engine.ProblemRequiresAPI}
	}
	if s.EchoFunctions != nil && !enabled.Any(engine.SettingEchoFunctions.Serves()...) {
		return nil, &engine.ConfigError{Setting: engine.SettingEchoFunctions, Problem: engine.ProblemServesNoRule}
	}
	var rules []engine.Rule
	for _, rule := range catalog.Rules {
		if enabled.Has(rule) {
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

// exemptions validates api-exempt and supplies the default. An absent key
// takes the default and an explicitly empty list exempts nothing. Either list
// without api patterns is rejected rather than ignored: there is no surface
// for it to speak to, so the configuration does not mean what it says.
func exemptions(s Settings) ([]engine.Kind, error) {
	if len(s.API) == 0 {
		if s.APIExempt != nil {
			return nil, &engine.ConfigError{Setting: engine.SettingAPIExempt, Problem: engine.ProblemRequiresAPI}
		}
		return nil, nil
	}
	if s.APIExempt == nil {
		return []engine.Kind{engine.KindMethod}, nil
	}

	exempts := make([]engine.Kind, 0, len(s.APIExempt))
	for _, name := range s.APIExempt {
		kind, ok := engine.ParseKind(name)
		if !ok {
			return nil, &engine.ConfigError{Setting: engine.SettingAPIExempt, Problem: engine.ProblemUnknownValue, Value: name}
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
		return 0, nil
	}
	if len(s.API) == 0 {
		return 0, &engine.ConfigError{Setting: engine.SettingAPIGenerics, Problem: engine.ProblemRequiresAPI}
	}
	rooting, ok := engine.ParseGenericRooting(*s.APIGenerics)
	if !ok {
		return 0, &engine.ConfigError{
			Setting: engine.SettingAPIGenerics, Problem: engine.ProblemUnknownValue, Value: *s.APIGenerics,
		}
	}
	return rooting, nil
}
