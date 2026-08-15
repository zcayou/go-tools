package engine

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// apiSurface is the declared public API: the packages whose exported surface
// consumers reach, and the declaration kinds that surface exempts. An exempt
// declaration is still a call-graph root, so what it alone reaches stays live.
type apiSurface struct {
	patterns []string
	packages map[string]bool
	exempts  map[Kind]bool
}

// newAPISurface validates the declared surface. Exemptions without patterns
// are rejected rather than silently ignored: nothing would be exempt, so
// the caller asked for something it is not getting.
func newAPISurface(patterns []string, exempts []Kind) (*apiSurface, error) {
	surface := &apiSurface{exempts: map[Kind]bool{}}
	for _, pattern := range patterns {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			surface.patterns = append(surface.patterns, pattern)
		}
	}
	for _, exempt := range exempts {
		if !slices.Contains(Kinds(), exempt) {
			return nil, fmt.Errorf("unknown api exemption %q: want one of %s", exempt, kindList())
		}
		surface.exempts[exempt] = true
	}
	if len(surface.patterns) == 0 && len(surface.exempts) > 0 {
		return nil, errors.New("api exemptions require api patterns")
	}
	return surface, nil
}

func kindList() string {
	names := make([]string, 0, len(Kinds()))
	for _, kind := range Kinds() {
		names = append(names, string(kind))
	}
	return strings.Join(names, ", ")
}

// resolve turns the API patterns into the package paths they match.
func (s *apiSurface) resolve(load *packages.Config) error {
	if len(s.patterns) == 0 {
		return nil
	}
	resolved, err := resolvePatterns(load, "api", s.patterns)
	if err != nil {
		return err
	}
	s.packages = resolved
	return nil
}

// resolvePatterns turns package patterns into the paths they match, loading
// names only. Patterns resolve against the same build configuration
// as the analysis so that build tags apply.
func resolvePatterns(load *packages.Config, what string, patterns []string) (map[string]bool, error) {
	names := &packages.Config{
		Mode:       packages.NeedName,
		Context:    load.Context,
		Env:        load.Env,
		BuildFlags: load.BuildFlags,
		Dir:        load.Dir,
	}
	matched, err := packages.Load(names, patterns...)
	if err != nil {
		return nil, fmt.Errorf("resolving %s packages: %w", what, err)
	}
	resolved := map[string]bool{}
	for _, pkg := range matched {
		// A pattern that resolves to nothing still comes back as a package:
		// the pattern text stands in for the path and the reason sits in Errors.
		// Recorded as-is it would be a path no declaration can match, leaving
		// the declared set silently covering nothing.
		if len(pkg.Errors) > 0 {
			errs := make([]error, 0, len(pkg.Errors))
			for _, e := range pkg.Errors {
				errs = append(errs, e)
			}
			return nil, fmt.Errorf("resolving %s pattern %s: %w", what, pkg.PkgPath, errors.Join(errs...))
		}
		if pkg.PkgPath != "" {
			resolved[pkg.PkgPath] = true
		}
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("%s %s matched no packages", what, strings.Join(patterns, ","))
	}
	return resolved, nil
}

// shields reports whether the declared surface covers this declaration at all.
// An unexported declaration is never covered however its kind is configured: no
// consumer can name it, so standing in for consumers this run cannot see says
// nothing about it.
func (s *apiSurface) shields(decl declaration) bool {
	return decl.exported && s.packages[decl.pkg] && s.exempts[decl.kind]
}

// exempted reports whether the declaration is shielded by the declared API
// surface. A method is shielded only while its receiver type is itself
// referenced: once the type is reported as unused, everything hanging off
// it is dead with it.
func (s *apiSurface) exempted(decl declaration, deadTypes map[string]bool) bool {
	if !s.shields(decl) {
		return false
	}
	return decl.kind != KindMethod || !deadTypes[decl.owner]
}
