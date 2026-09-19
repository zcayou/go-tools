package main

import "fmt"

// Box is converted to fmt.Stringer, so RTA registers it as a runtime type
// and walks its method set, generic methods included.
type Box struct{ v int }

func (b Box) String() string { return fmt.Sprint(b.v) }

// Map is called with an explicit type argument.
func (b Box) Map[T any](f func(int) T) T { return f(b.v) }

// Unused is never called, and onlyFromUnused is reachable through it alone.
func (b Box) Unused[T any](x T) T { return onlyFromUnused(x) }

func onlyFromUnused[T any](x T) T { return x }

type List[E any] struct{ items []E }

func (l *List[E]) Push(e E) { l.items = append(l.items, e) }

// Convert declares type parameters of its own on a generic type.
func (l *List[E]) Convert[F any](f func(E) F) *List[F] {
	out := &List[F]{}
	for _, e := range l.items {
		out.Push(f(e))
	}
	return out
}

func identity[T any](x T) T { return x }

func main() {
	b := Box{v: 1}
	fmt.Println(b.Map[string](func(i int) string { return fmt.Sprint(i) }))
	var s fmt.Stringer = b
	fmt.Println(s)
	l := &List[int]{}
	l.Push(1)
	fmt.Println(l.Convert(func(i int) string { return fmt.Sprint(i) }).items)
	// The assignment alone infers identity's type argument.
	var f func(int) int = identity
	fmt.Println(f(2))
}
