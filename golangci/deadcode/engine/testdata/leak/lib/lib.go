// Package lib holds four vocabularies. Production compares three and renders
// none of them; each of those three is rendered by a test, from a different
// place. The fourth production renders from a package-level initializer.
package lib

import "fmt"

// Phase is rendered by an in-package test file's package-level initializer.
type Phase uint8

const (
	Start Phase = iota + 1
	Stop
)

func (p Phase) String() string { return [...]string{"", "start", "stop"}[p] }

// Mode is rendered by an external test package's package-level initializer.
type Mode uint8

const ModeOn Mode = 1

func (m Mode) String() string { return "on" }

// Level is rendered inside a test function.
type Level uint8

const High Level = 1

func (l Level) String() string { return "high" }

// Shown is rendered by production's own package-level initializer, which
// the in-package test variant's initializer runs too.
type Shown uint8

const ShownOn Shown = 1

func (s Shown) String() string { return "shown" }

var shown = fmt.Sprint(ShownOn)

// Describe returns what production rendered.
func Describe() string { return shown }

// Done compares, and renders nothing.
func Done(p Phase, m Mode, l Level) bool { return p == Stop && m == ModeOn && l == High }
