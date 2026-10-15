package ztest

import (
	"errors"
	"slices"
	"sync"

	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/ztest/layout"
	"github.com/zcayou/go-tools/golangci/ztest/spec"
)

const analyzerDoc = `checks where tests live, what specs assert, and the context they run under

A test file is white-box or black-box depending on whether it declares the
package under test or the external <pkg>_test package, or standalone where its
directory holds no source to test; each kind is allowed or not and named after
a configured set of patterns. A file named for a source file carries the specs
for it; a file named for a supporting role carries no specs. Where Ginkgo is
used, the adapter that calls RunSpecs sits in a file reserved for it, and it
exists. A fixture lives in the one file consuming it, or in a helper file when
more than one does.

A spec asserts which error it got, not merely that one occurred, and an error's
text and an enum's tokens are each asserted once, in the test file pinning the
rendering of the source that declares them. A test takes the context its
framework hands it rather than context.Background.`

// NewAnalyzer builds the analyzer that runs every enabled rule under settings.
// It fails with a [*ConfigError] on settings no run could satisfy, and reads
// nothing from disk.
//
// golangci-lint attributes every analyzer a plugin returns to the plugin's
// name, but prefixes a message with the analyzer's own name whenever the two
// differ, which would put a word ahead of the rule token every message opens
// with. So both halves run inside one analyzer named for the linter.
func NewAnalyzer(settings Settings) (*analysis.Analyzer, error) {
	config, err := Resolve(settings)
	if err != nil {
		return nil, err
	}

	var halves []*analysis.Analyzer
	if len(config.Layout.Disabled) < len(layoutRules) {
		halves = append(halves, layout.NewAnalyzer(config.Layout))
	}
	if len(config.Spec.Disabled) < len(specRules) {
		halves = append(halves, spec.NewAnalyzer(config.Spec))
	}

	analyzer := &analysis.Analyzer{
		Name: Name,
		Doc:  analyzerDoc,
		Run: func(pass *analysis.Pass) (any, error) {
			for _, half := range halves {
				if _, err := half.Run(pass); err != nil {
					return nil, err
				}
			}
			return nil, nil
		},
	}
	for _, half := range halves {
		for _, required := range half.Requires {
			if !slices.Contains(analyzer.Requires, required) {
				analyzer.Requires = append(analyzer.Requires, required)
			}
		}
		analyzer.FactTypes = append(analyzer.FactTypes, half.FactTypes...)
	}
	return analyzer, nil
}

// apiScope is an explicit api setting. Only the module can say what its
// patterns match, so they are resolved from the module root, once
// for the process, by the first pass that asks.
type apiScope struct {
	// patterns are the configured api patterns.
	patterns []string

	// buildTags are the build tags the patterns resolve under.
	buildTags []string

	once  sync.Once
	paths map[string]bool
	err   error
}

var _ spec.Scope = (*apiScope)(nil)

// Includes reports whether the patterns match the package at path.
func (s *apiScope) Includes(path string) (bool, error) {
	s.once.Do(s.resolve)
	if s.err != nil {
		return false, s.err
	}
	return s.paths[path], nil
}

// resolve matches the configured patterns against the module. The patterns
// are its own to answer for, so whatever stops them resolving is reported
// against the api setting.
func (s *apiScope) resolve() {
	root, err := wholemodule.ModuleRoot()
	if err != nil {
		s.err = &ConfigError{Setting: SettingAPI, Problem: ProblemNoModuleRoot, cause: err}
		return
	}
	paths, err := wholemodule.ResolveAPI(root, s.buildTags, s.patterns)
	if err != nil {
		e := &ConfigError{Setting: SettingAPI, Problem: ProblemUnresolvedAPIPattern, cause: err}
		if pattern, ok := errors.AsType[*wholemodule.PatternError](err); ok {
			e.Pattern = pattern.Pattern
		}
		s.err = e
		return
	}
	s.paths = paths
}
