// Package lib is the declared API. Its sealed generic interfaces reach consumers
// only through generic methods of Pipe, so which instantiation each is weighed
// against turns on which parameters the method's signature mentions.
package lib

// Pipe is generic, and NewPipe builds the module's one instantiation of it.
type Pipe[E comparable] struct{}

func NewPipe() Pipe[int] { return Pipe[int]{} }

// Taken is exposed mentioning only Pipe's parameter, so it is weighed
// at Pipe[int], though Drain itself is instantiated nowhere.
type Taken[K comparable] interface{ taken() K }

type IntTaken struct{ value int }

func (i IntTaken) taken() int { return i.value }

func (Pipe[E]) Drain[F any](f F) Taken[E] { return nil }

// Keyed is exposed mentioning only the method's own parameter.
type Keyed[K comparable] interface{ key() K }

type StringKey struct{ value string }

func (s StringKey) key() string { return s.value }

func (Pipe[E]) Keys[F comparable]() Keyed[F] { return nil }

// Paired is exposed mentioning both, which only the method's instantiation
// pairs.
type Paired[K comparable, V any] interface{ pair() (K, V) }

type IntString struct{}

func (IntString) pair() (int, string) { return 0, "" }

func (Pipe[E]) Pairs[F any]() Paired[E, F] { return nil }

func Use() {
	_ = NewPipe().Keys[string]()
	_ = NewPipe().Pairs[string]()
}
