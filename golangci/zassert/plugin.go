package zassert

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/internal/filepattern"
)

// Name is what golangci-lint knows this linter by: the key under
// linters.settings.custom, the entry in linters.enable, and the token
// a //nolint:zassert directive must carry.
const Name = "zassert"

// Settings is the decoded linters.settings.custom.zassert.settings block.
//
// Decoding rejects unknown fields, so a misspelled key fails the run rather
// than being silently ignored. The zero value is the default configuration:
// every API package's specs are judged by unidentified-error, and a test file
// named for a source declaring an Error method pins that rendering.
type Settings struct {
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

// DefaultRenderingPatterns are the names of the test files that pin a rendering
// when settings do not name their own: the spec named for the source
// it exercises.
func DefaultRenderingPatterns() []string {
	return []string{filepattern.SourceToken + "_test.go"}
}

// Plugin adapts the analyzer to golangci-lint's module plugin contract.
type Plugin struct {
	analyzer *analysis.Analyzer
}

var _ register.LinterPlugin = (*Plugin)(nil)

// New builds the plugin from the raw settings golangci-lint decoded out
// of the configuration file. It satisfies [register.NewPlugin]. It reads
// nothing from disk: an explicit api setting is resolved by the run, which
// is where a pattern matching nothing fails.
func New(settings any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](settings)
	if err != nil {
		return nil, &ConfigError{Problem: ProblemUndecodableSettings, cause: err}
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

// GetLoadMode reports that the rules need type information: every name
// resolves through it, and an enum is known by the fact its declaring
// package's pass exports.
func (p *Plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

// config is [Settings] with the defaults filled in and every pattern compiled.
type config struct {
	api        *apiScope
	renderings filepattern.Set
}

// resolve fills in the defaults and compiles the rendering patterns, rejecting
// a configuration no run could satisfy. Nothing here needs the module, so
// what only the module can answer waits for the run.
func (s Settings) resolve() (config, error) {
	if s.API != nil && len(s.API) == 0 {
		return config{}, &ConfigError{Setting: SettingAPI, Problem: ProblemEmptyAPI}
	}

	texts := s.RenderingPatterns
	if texts == nil {
		texts = DefaultRenderingPatterns()
	}
	renderings := make(filepattern.Set, 0, len(texts))
	for _, text := range texts {
		p, err := filepattern.New(text)
		if err != nil {
			return config{}, &ConfigError{
				Setting: SettingRenderingPatterns, Problem: ProblemMalformedRenderingPattern, Pattern: text, cause: err,
			}
		}
		if !p.NamesSource() {
			return config{}, &ConfigError{
				Setting: SettingRenderingPatterns, Problem: ProblemSourcelessRenderingPattern, Pattern: text,
			}
		}
		renderings = append(renderings, p)
	}

	return config{api: &apiScope{patterns: s.API, buildTags: s.BuildTags}, renderings: renderings}, nil
}
