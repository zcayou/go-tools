package main

import "fmt"

// Register is called from a blank var whose value is computed, so the call runs and the reference
// to it counts.
func Register(name string) int { return len(name) }

var _ = Register("thing")

// Marker is named only by a compile-time assertion, which is not a use of it or of Widget.
type Marker interface{ Mark() }

type Widget struct{}

func (Widget) Mark() {}

var _ Marker = Widget{}

// Hooks carries a function literal into an inert assertion, where the literal's interior is still
// live source however inert the spec around it is.
type Hooks struct{ Run func() string }

func (Hooks) Mark() {}

var _ Marker = Hooks{Run: func() string { return Hooked() }}

// Hooked is named only from inside that literal.
func Hooked() string { return "hooked" }

// Node is asserted in the two other shapes an assertion is written in, a conversion of nil and
// the address of a composite literal, both of which the compiler satisfies without running
// anything.
type Node struct{}

func (*Node) Mark() {}

var (
	_ Marker = (*Node)(nil)
	_ Marker = &Node{}
)

// Helper is named only inside a function literal, and that body runs.
func Helper() string { return "helper" }

var _ = describe(func() { fmt.Println(Helper()) })

func describe(body func()) int {
	body()
	return 0
}

func main() {}
