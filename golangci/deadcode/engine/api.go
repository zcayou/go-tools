package engine

import (
	"cmp"
	"errors"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// apiSurface is the declared public API: the packages whose exported surface
// consumers reach, the declaration kinds that surface exempts, and how its
// generic declarations are rooted. An exempt declaration is still a call-graph
// root, so what it alone reaches stays live.
type apiSurface struct {
	patterns []string
	packages map[string]bool
	exempts  map[Kind]bool
	generics GenericRooting
}

// newAPISurface validates the declared surface. Exemptions and a generic
// rooting without patterns are rejected rather than silently ignored: nothing
// would be exempt and nothing would be rooted, so the caller asked
// for something it is not getting.
func newAPISurface(patterns []string, exempts []Kind, generics GenericRooting) (*apiSurface, error) {
	if generics != 0 && !slices.Contains(GenericRootings(), generics) {
		return nil, &ConfigError{Setting: SettingAPIGenerics, Problem: ProblemUnknownValue, Value: generics.String()}
	}
	surface := &apiSurface{exempts: map[Kind]bool{}}
	for _, pattern := range patterns {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			surface.patterns = append(surface.patterns, pattern)
		}
	}
	for _, exempt := range exempts {
		if !slices.Contains(Kinds(), exempt) {
			return nil, &ConfigError{Setting: SettingAPIExempt, Problem: ProblemUnknownValue, Value: exempt.String()}
		}
		surface.exempts[exempt] = true
	}
	if len(surface.patterns) == 0 {
		if len(surface.exempts) > 0 {
			return nil, &ConfigError{Setting: SettingAPIExempt, Problem: ProblemRequiresAPI}
		}
		if generics != 0 && generics != GenericRootingSkip {
			return nil, &ConfigError{Setting: SettingAPIGenerics, Problem: ProblemRequiresAPI}
		}
		surface.generics = GenericRootingSkip
		return surface, nil
	}
	// A declared surface that is generic is the case this exists for, so measuring
	// it is what an unset rooting asks for. Under skip the analysis roots none
	// of it, which is a claim the run cannot support and reports as dead code.
	surface.generics = cmp.Or(generics, GenericRootingInstantiated)
	return surface, nil
}

// rootsInstantiations reports whether the surface's generic declarations enter
// the root set through the instantiations the program builds of them.
func (s *apiSurface) rootsInstantiations() bool {
	return s.generics == GenericRootingInstantiated
}

// resolve turns the API patterns into the package paths they match.
func (s *apiSurface) resolve(load *packages.Config) error {
	if len(s.patterns) == 0 {
		return nil
	}
	resolved, err := resolvePatterns(load, SettingAPI, s.patterns)
	if err != nil {
		return err
	}
	s.packages = resolved
	return nil
}

// resolvePatterns turns package patterns into the paths they match, loading
// names only. Patterns resolve against the same build configuration
// as the analysis so that build tags apply.
func resolvePatterns(load *packages.Config, setting Setting, patterns []string) (map[string]bool, error) {
	names := &packages.Config{
		Mode:       packages.NeedName,
		Context:    load.Context,
		Env:        load.Env,
		BuildFlags: load.BuildFlags,
		Dir:        load.Dir,
	}
	matched, err := packages.Load(names, patterns...)
	if err != nil {
		return nil, &ConfigError{Setting: setting, Problem: ProblemUnresolvable, Value: strings.Join(patterns, ","), cause: err}
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
			return nil, &ConfigError{
				Setting: setting, Problem: ProblemUnresolvable, Value: pkg.PkgPath, cause: errors.Join(errs...),
			}
		}
		if pkg.PkgPath != "" {
			resolved[pkg.PkgPath] = true
		}
	}
	if len(resolved) == 0 {
		return nil, &ConfigError{Setting: setting, Problem: ProblemMatchesNothing, Value: strings.Join(patterns, ",")}
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
