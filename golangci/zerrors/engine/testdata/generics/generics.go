// Package generics declares methods with type parameters of their own,
// a generic interface consumers call, and a generic function returning what
// its type argument makes.
package generics

import "errors"

// Box has a method with type parameters of its own.
type Box struct{}

func (Box) Get[T any](T) error { return errors.New("box") }

func UseBox() error { return Box{}.Get[int](1) }

func AnyBox() any { return Box{} }

// Set is generic, with a method generic over more.
type Set[E any] struct{}

func (*Set[E]) Map[R any](func(E) R) error { return errors.New("set") }

func AnySet() any { return &Set[int]{} }

// Getter is a generic interface consumers call.
type Getter[T any] interface{ Get() (T, error) }

type intGetter struct{}

func (intGetter) Get() (int, error) { return 0, errors.New("int") }

func NewGetter() Getter[int] { return intGetter{} }

type raw struct{}

func (raw) Error() string { return "raw" }

func Wrap[E error](e E) error { return e }

func WrapRaw() error { return Wrap(raw{}) }
