// Package filepattern is the grammar test-file names are configured in, shared
// by the linters that judge a file by its name.
//
// A pattern is either a shell glob, matched against a file name, or a name
// holding [SourceToken] once, which stands for the base name of a source file
// in the same directory. "<source>_test.go" means "named after the source
// it exercises" rather than naming any file in particular.
package filepattern

import (
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
)

// SourceToken stands, inside a pattern, for the base name of a source file
// in the same directory.
const SourceToken = "<source>"

// Pattern is one name a file may have. A pattern either resolves [SourceToken]
// against the directory or is a shell glob; it is not both, because the token
// has to be matched to be resolved and a glob does not report what it matched.
type Pattern struct {
	text string

	// prefix and suffix bracket SourceToken and are matched literally. They
	// are empty for a glob pattern, which source distinguishes.
	prefix string
	suffix string
	source bool
}

// New compiles one configured pattern. A pattern no file name could match —
// one holding a path separator, a malformed glob, a glob wrapped around
// [SourceToken], or a second [SourceToken] — is rejected with an [*Error]
// rather than left in the configuration matching nothing.
func New(text string) (Pattern, error) {
	if strings.ContainsAny(text, `/\`) {
		return Pattern{}, &Error{Pattern: text, Problem: ProblemPathSeparator}
	}
	switch strings.Count(text, SourceToken) {
	case 0:
		if _, err := path.Match(text, ""); err != nil {
			return Pattern{}, &Error{Pattern: text, Problem: ProblemMalformedGlob, cause: err}
		}
		return Pattern{text: text}, nil
	case 1:
		prefix, suffix, _ := strings.Cut(text, SourceToken)
		if strings.ContainsAny(prefix+suffix, "*?[") {
			return Pattern{}, &Error{Pattern: text, Problem: ProblemGlobAroundSource}
		}
		return Pattern{text: text, prefix: prefix, suffix: suffix, source: true}, nil
	default:
		return Pattern{}, &Error{Pattern: text, Problem: ProblemRepeatedSource}
	}
}

// Error is a pattern [New] rejects.
type Error struct {
	// Pattern is the pattern as it was configured.
	Pattern string
	// Problem is why no file name could match it.
	Problem Problem

	// cause is what [path.Match] said of a malformed glob.
	cause error
}

func (e *Error) Error() string {
	switch e.Problem {
	case ProblemPathSeparator:
		return fmt.Sprintf("pattern %q: matched against a file name, which holds no path separator", e.Pattern)
	case ProblemMalformedGlob:
		if e.cause == nil {
			return fmt.Sprintf("pattern %q: malformed glob", e.Pattern)
		}
		return fmt.Sprintf("pattern %q: %v", e.Pattern, e.cause)
	case ProblemGlobAroundSource:
		return fmt.Sprintf("pattern %q: %s is matched literally, so it cannot be combined with a glob",
			e.Pattern, SourceToken)
	case ProblemRepeatedSource:
		return fmt.Sprintf("pattern %q: %s may appear once", e.Pattern, SourceToken)
	default:
		return fmt.Sprintf("pattern %q: %s", e.Pattern, e.Problem)
	}
}

// Unwrap returns [path.ErrBadPattern] for a malformed glob, and nil otherwise.
func (e *Error) Unwrap() error {
	return e.cause
}

// Problem is why [New] rejected a pattern.
type Problem int

const (
	// ProblemPathSeparator is a pattern holding a path separator, which a file
	// name never does.
	ProblemPathSeparator Problem = iota + 1
	// ProblemMalformedGlob is a glob [path.Match] cannot parse.
	ProblemMalformedGlob
	// ProblemGlobAroundSource is a glob wrapped around [SourceToken], which
	// is matched literally.
	ProblemGlobAroundSource
	// ProblemRepeatedSource is a pattern holding [SourceToken] more than once.
	ProblemRepeatedSource
)

func (p Problem) String() string {
	switch p {
	case ProblemPathSeparator:
		return "path-separator"
	case ProblemMalformedGlob:
		return "malformed-glob"
	case ProblemGlobAroundSource:
		return "glob-around-source"
	case ProblemRepeatedSource:
		return "repeated-source"
	default:
		return "Problem(" + strconv.Itoa(int(p)) + ")"
	}
}

// String is the pattern as it was configured.
func (p Pattern) String() string {
	return p.text
}

// NamesSource reports whether the pattern holds [SourceToken].
func (p Pattern) NamesSource() bool {
	return p.source
}

// Matches reports whether name is a name this pattern allows, resolving
// [SourceToken] against the source files sitting beside it.
func (p Pattern) Matches(name string, srcs Sources) bool {
	if !p.source {
		ok, _ := path.Match(p.text, name)
		return ok
	}
	base, ok := p.Source(name)
	return ok && srcs.Declares(base)
}

// Source returns the base name name gives [SourceToken], whether or not
// a source of that name exists. It reports false for a glob pattern,
// and for a name the pattern's literal parts do not bracket.
func (p Pattern) Source(name string) (string, bool) {
	if !p.source {
		return "", false
	}
	base, ok := between(name, p.prefix, p.suffix)
	return base, ok && base != ""
}

// between returns what sits between prefix and suffix in s.
func between(s, prefix, suffix string) (string, bool) {
	if len(s) < len(prefix)+len(suffix) ||
		!strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return "", false
	}
	return s[len(prefix) : len(s)-len(suffix)], true
}

// Set is every name one setting allows.
type Set []Pattern

// NewSet compiles every pattern of one setting, failing on the first that [New]
// rejects.
func NewSet(texts []string) (Set, error) {
	set := make(Set, 0, len(texts))
	for _, text := range texts {
		p, err := New(text)
		if err != nil {
			return nil, err
		}
		set = append(set, p)
	}
	return set, nil
}

// Matches reports whether any pattern of the set allows name.
func (s Set) Matches(name string, srcs Sources) bool {
	return slices.ContainsFunc(s, func(p Pattern) bool { return p.Matches(name, srcs) })
}

// String lists the names the set allows, for a diagnostic that has to say what
// a file could have been called instead.
func (s Set) String() string {
	texts := make([]string, len(s))
	for i, p := range s {
		texts[i] = p.text
	}
	return strings.Join(texts, ", ")
}
