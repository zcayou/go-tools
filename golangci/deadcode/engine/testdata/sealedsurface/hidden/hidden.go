// Package hidden holds the same sealed shape outside any declared surface: with no consumers
// declared for it, nothing stands in for the ones it does not have.
package hidden

type Gate interface{ gate() string }

type Entry struct{ name string }

func (e Entry) gate() string { return e.name }

func Pass(g Gate) string { return g.gate() }
