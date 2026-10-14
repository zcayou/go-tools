package wholemodule

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ResolveAPI returns the import paths of the module's API packages:
// the packages of the module rooted at root that patterns match, which are not
// main and hold no internal path element. Go already stops any other module
// importing an internal package, so no pattern makes one API, and a package
// outside the module, from the standard library or a dependency, is no part
// of its API either. nil patterns mean ./...
//
// Each pattern is resolved on its own, so that one matching no API package
// of the module fails with a [*PatternError] naming it rather than leaving
// the configuration covering less than it says.
func ResolveAPI(root string, buildTags, patterns []string) (map[string]bool, error) {
	return resolve(root, buildTags, patterns, Public)
}

// ResolvePackages returns the import paths of the packages of the module rooted
// at root that patterns match, main and internal ones included. nil patterns
// mean ./... Each pattern has to match a package of the module, or it fails
// with a [*PatternError] naming it.
func ResolvePackages(root string, buildTags, patterns []string) (map[string]bool, error) {
	return resolve(root, buildTags, patterns, func(string, string) bool { return true })
}

// resolve returns the module packages patterns match that admit takes, by path
// and name. A pattern matching only packages admit refuses fails with them
// as its Excluded.
func resolve(root string, buildTags, patterns []string, admit func(path, name string) bool) (map[string]bool, error) {
	if patterns == nil {
		patterns = []string{"./..."}
	}
	config := &packages.Config{Mode: packages.NeedName | packages.NeedModule, Dir: root}
	if len(buildTags) > 0 {
		config.BuildFlags = []string{"-tags=" + strings.Join(buildTags, ",")}
	}
	moduleDir := canonicalDir(root)

	resolved := map[string]bool{}
	for _, pattern := range patterns {
		matched, err := packages.Load(config, pattern)
		if err != nil {
			return nil, &PatternError{Pattern: pattern, cause: err}
		}
		var excluded, foreign []string
		found := false
		for _, pkg := range matched {
			// A pattern that resolves to nothing can still come back as a package:
			// the pattern text stands in for the path and the reason sits in Errors.
			if len(pkg.Errors) > 0 {
				errs := make([]error, 0, len(pkg.Errors))
				for _, e := range pkg.Errors {
					errs = append(errs, e)
				}
				return nil, &PatternError{Pattern: pattern, cause: errors.Join(errs...)}
			}
			switch {
			case pkg.Module == nil || canonicalDir(pkg.Module.Dir) != moduleDir:
				foreign = append(foreign, pkg.PkgPath)
			case !admit(pkg.PkgPath, pkg.Name):
				excluded = append(excluded, pkg.PkgPath)
			default:
				resolved[pkg.PkgPath] = true
				found = true
			}
		}
		if !found {
			slices.Sort(excluded)
			slices.Sort(foreign)
			return nil, &PatternError{Pattern: pattern, Excluded: excluded, Foreign: foreign}
		}
	}
	return resolved, nil
}

// canonicalDir is dir with symbolic links resolved, so that a module root
// reached through a link compares equal to the directory the loader reports.
// A directory that cannot be resolved compares as written.
func canonicalDir(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return filepath.Clean(dir)
}

// Public reports whether a package with this import path and name can
// be imported by another module, which is what makes it a candidate
// for the API: it is not main, and its path holds no internal element.
func Public(path, name string) bool {
	if name == "main" {
		return false
	}
	for element := range strings.SplitSeq(path, "/") {
		if element == "internal" {
			return false
		}
	}
	return true
}

// PatternError is a package pattern that matches no package of the module
// it was resolved for.
type PatternError struct {
	// Pattern is the pattern as it was configured.
	Pattern string

	// Excluded are the packages of the module the pattern did match, each
	// of them main or internal and so not API.
	Excluded []string

	// Foreign are the packages outside the module the pattern did match:
	// standard library packages and those of dependencies.
	Foreign []string

	// cause is what the loader said, when it could not resolve the pattern.
	cause error
}

func (e *PatternError) Error() string {
	switch {
	case e.cause != nil:
		return fmt.Sprintf("resolving pattern %s: %v", e.Pattern, e.cause)
	case len(e.Excluded) > 0:
		return fmt.Sprintf("pattern %s matched only main or internal packages, which are not API: %s",
			e.Pattern, strings.Join(e.Excluded, ", "))
	case len(e.Foreign) > 0:
		return fmt.Sprintf("pattern %s matched only packages outside the module: %s",
			e.Pattern, strings.Join(e.Foreign, ", "))
	default:
		return fmt.Sprintf("pattern %s matched no packages", e.Pattern)
	}
}

// Unwrap returns what the loader said, when it could not resolve the pattern.
func (e *PatternError) Unwrap() error {
	return e.cause
}
