package testlayout

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// sourceToken stands, inside a pattern, for the base name of a source file
// in the same directory. It is what lets "<source>_test.go" mean "named after
// the source it exercises" rather than naming any file in particular.
const sourceToken = "<source>"

// pattern is one name a test file may have. A pattern either resolves
// [sourceToken] against the directory or is a shell glob; it is not both,
// because the token has to be matched to be resolved and a glob does not report
// what it matched.
type pattern struct {
	text string

	// prefix and suffix bracket sourceToken and are matched literally. They
	// are empty for a glob pattern, which source distinguishes.
	prefix string
	suffix string
	source bool
}

// newPattern compiles one configured pattern. A pattern no file name could
// match — one holding a path separator, a malformed glob, a glob wrapped around
// [sourceToken], or a second [sourceToken] — is rejected rather than left
// in the configuration matching nothing.
func newPattern(text string) (pattern, error) {
	if strings.ContainsAny(text, `/\`) {
		return pattern{}, fmt.Errorf(
			"pattern %q: matched against a file name, which holds no path separator", text)
	}
	switch strings.Count(text, sourceToken) {
	case 0:
		if _, err := path.Match(text, ""); err != nil {
			return pattern{}, fmt.Errorf("pattern %q: %w", text, err)
		}
		return pattern{text: text}, nil
	case 1:
		prefix, suffix, _ := strings.Cut(text, sourceToken)
		if strings.ContainsAny(prefix+suffix, "*?[") {
			return pattern{}, fmt.Errorf(
				"pattern %q: %s is matched literally, so it cannot be combined with a glob",
				text, sourceToken)
		}
		return pattern{text: text, prefix: prefix, suffix: suffix, source: true}, nil
	default:
		return pattern{}, fmt.Errorf("pattern %q: %s may appear once", text, sourceToken)
	}
}

// matches reports whether name is a name this pattern allows, resolving
// [sourceToken] against the source files sitting beside it.
func (p pattern) matches(name string, srcs sources) bool {
	if !p.source {
		ok, _ := path.Match(p.text, name)
		return ok
	}
	base, ok := between(name, p.prefix, p.suffix)
	return ok && base != "" && srcs.declares(base)
}

// between returns what sits between prefix and suffix in s.
func between(s, prefix, suffix string) (string, bool) {
	if len(s) < len(prefix)+len(suffix) ||
		!strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return "", false
	}
	return s[len(prefix) : len(s)-len(suffix)], true
}

// patternSet is every name one rule allows.
type patternSet []pattern

func newPatternSet(texts []string) (patternSet, error) {
	set := make(patternSet, 0, len(texts))
	for _, text := range texts {
		p, err := newPattern(text)
		if err != nil {
			return nil, err
		}
		set = append(set, p)
	}
	return set, nil
}

func (ps patternSet) matches(name string, srcs sources) bool {
	return slices.ContainsFunc(ps, func(p pattern) bool { return p.matches(name, srcs) })
}

// String lists the names the set allows, for a diagnostic that has to say what
// a file could have been called instead.
func (ps patternSet) String() string {
	texts := make([]string, len(ps))
	for i, p := range ps {
		texts[i] = p.text
	}
	return strings.Join(texts, ", ")
}
