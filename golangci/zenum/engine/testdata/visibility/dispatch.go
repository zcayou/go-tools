package visibility

import "fmt"

// boxed is printed only by a method of a generic type that nothing calls but
// through an interface.
type boxed int

const boxedA boxed = 1

type shower interface{ show() string }

type box[T any] struct{ v T }

func (b box[T]) show() string { return fmt.Sprint(b.v) }

// keep has type parameters of its own, and so no method value.
func (b box[T]) keep[U any](u U) U { return u }

func boxIt() shower { return box[boxed]{boxedA} }
