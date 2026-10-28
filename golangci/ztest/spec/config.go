package spec

import (
	"strings"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/ztest/internal/filepattern"
)

// Config is what the spec rules judge test files under: whose specs owe
// an identified error, which test files pin a rendering and under which build
// tags, and which rules run. The ztest linter resolves it from its settings.
type Config struct {
	// API narrows the packages whose specs unidentified-error judges. Nil is every
	// API package: not main, and holding no internal path element.
	API Scope

	// Renderings are the names of the test files that pin the rendering
	// of the source they are named for, each holding [filepattern.SourceToken].
	Renderings filepattern.Set

	// BuildTags are the build tags beyond the platform's under which a test
	// file's build constraints decide whether it can pin a rendering.
	BuildTags []string

	// Disabled are rules that report nothing. Every rule runs by default.
	Disabled []Rule
}

// Scope decides which packages are API. Includes is given the import path
// of a package whose specs are judged, and may fail when deciding takes
// the module and the module cannot answer.
type Scope interface {
	Includes(path string) (bool, error)
}

// identifies reports whether the specs of pkg owe an identified error. A test
// package answers for the package it tests: the external foo_test package
// for foo.
func (c Config) identifies(path, name string) (bool, error) {
	if base, ok := strings.CutSuffix(name, "_test"); ok {
		path, name = strings.TrimSuffix(path, "_test"), base
	}
	if c.API == nil {
		return wholemodule.Public(path, name), nil
	}
	return c.API.Includes(path)
}
