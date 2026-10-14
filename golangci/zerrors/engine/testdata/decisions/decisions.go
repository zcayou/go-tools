// Package decisions decides on errors' text, and compares it in ways that
// decide nothing.
package decisions

import (
	"fmt"
	"regexp"
	"strings"
)

// Failure is a module error type.
type Failure struct {
	Reason string
	Code   int
}

func (f *Failure) Error() string { return f.Reason }

// Classified embeds a type whose string field it holds.
type Classified struct{ detail }

type detail struct{ Kind string }

func (*Classified) Error() string { return "classified" }

func contains(err error) bool { return strings.Contains(err.Error(), "x") }

func prefix(err error) bool { return strings.HasPrefix(err.Error(), "x") }

var pattern = regexp.MustCompile("x")

func matches(err error) bool { return pattern.MatchString(err.Error()) }

func equal(err error) bool { return err.Error() == "x" }

func notEqual(err error) bool { return err.Error() != "x" }

func switched(err error) int {
	switch err.Error() {
	case "a":
		return 1
	}
	return 0
}

func sprint(err error) bool { return fmt.Sprint(err) == "x" }

func sprintf(err error) bool { return strings.Contains(fmt.Sprintf("%v", err), "x") }

func field(f *Failure) bool { return f.Reason == "x" }

func embedded(c *Classified) bool { return c.Kind == "x" }

func direct(f *Failure) bool { return f.Error() == "x" }

func transformed(err error) bool { return strings.ToLower(err.Error()) == "x" }

func local(err error) bool {
	text := ""
	if err != nil {
		text = err.Error()
	}
	return text == "x"
}

func empty(err error) bool { return err.Error() == "" }

func both(a, b error) bool { return a.Error() == b.Error() }

func ordered(a, b error) int { return strings.Compare(a.Error(), b.Error()) }

func code(f *Failure) bool { return f.Code == 1 }

var _ = []any{contains, prefix, matches, equal, notEqual, switched, sprint, sprintf, field, embedded, direct,
	transformed, local, empty, both, ordered, code}
