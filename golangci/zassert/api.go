package zassert

import (
	"errors"
	"go/types"
	"strings"
	"sync"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// apiScope decides which packages' specs unidentified-error judges.
//
// Under the default, whether a package is API is decidable from the package
// alone: it is not main and has no internal path element, and ./... matches
// every package a run lints. So the default reads nothing and loads nothing.
// Explicit patterns narrow that, and only the module can say what they match,
// so they are resolved from the module root, once for the process, by the first
// pass that asks.
type apiScope struct {
	// patterns are the configured api patterns, or nil for the default.
	patterns []string

	// buildTags are the build tags the patterns resolve under.
	buildTags []string

	once  sync.Once
	paths map[string]bool
	err   error
}

// includes reports whether pkg's specs are judged. A test package answers
// for the package it tests: the external foo_test package for foo.
func (s *apiScope) includes(pkg *types.Package) (bool, error) {
	path, name := pkg.Path(), pkg.Name()
	if base, ok := strings.CutSuffix(name, "_test"); ok {
		path, name = strings.TrimSuffix(path, "_test"), base
	}

	if s.patterns == nil {
		return wholemodule.Public(path, name), nil
	}
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
