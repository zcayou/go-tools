package main

import "fmt"

// Ghost is referenced by nothing. fmt asserts Stringer and error in its own source, so an
// assertion that is not weighed against the types the program actually materializes would credit
// these two methods away — and every program imports fmt.
type Ghost struct{}

func (Ghost) String() string { return "ghost" }
func (Ghost) Error() string  { return "ghost" }

// bucket is a local interface nothing asserts, selected on below.
type bucket interface{ Push(int) }

// Bucket satisfies bucket structurally and is never held behind an interface, so the selection
// in drain is no evidence about it.
type Bucket struct{}

func (Bucket) Push(n int) { fmt.Println(n) }

func drain(b bucket) { b.Push(1) }

func main() {
	drain(nil)
}
