// Package lib is the declared API. Its interfaces are sealed, so the conversion that puts a value
// behind one happens in a consumer this module cannot load.
package lib

// Input is sealed by an unexported method, which no other package can supply. Nothing here converts
// a handle to it: a consumer does that when it calls Consume.
type Input interface{ input() token }

type token struct{ id string }

// Ref is the handle a consumer builds and passes to Consume.
type Ref struct{ id string }

func (r Ref) input() token { return token{id: r.id} }

// Tagged carries a phantom parameter, so the credit has to reach a method whose receiver is generic.
type Tagged[T any] struct {
	_  [0]*T
	id string
}

func (t Tagged[T]) input() token { return token{id: t.id} }

// Bound is sealed and declares an exported method alongside the seal, which a consumer holding
// the interface can call.
type Bound interface {
	bound() token
	Name() string
}

type Binding struct{ id string }

func (b Binding) bound() token { return token{id: b.id} }

func (b Binding) Name() string { return b.id }

// halfBound carries the seal alone, so it implements Bound nowhere and the seal credits it nowhere.
type halfBound struct{ id string }

func (h halfBound) bound() token { return token{id: h.id} }

// Keyed is a sealed generic interface, so what a consumer can hold is decided one instantiation
// at a time.
type Keyed[K comparable] interface{ key() K }

type StringKey struct{ value string }

func (s StringKey) key() string { return s.value }

// Untaken is sealed and generic and the program instantiates it nowhere, so there is no concrete
// interface to weigh an implementation against.
type Untaken[K comparable] interface{ taken() K }

type IntTaken struct{ value int }

func (i IntTaken) taken() int { return i.value }

// Open is not sealed: an exported method set is one any package can satisfy, so its implementations
// are weighed as they always were.
type Open interface{ Label() string }

type Plain struct{}

func (Plain) Label() string { return "plain" }

func Consume(in Input) string { return in.input().id }

func Describe(b Bound) string { return b.bound().id + b.Name() }

func LookupString(k Keyed[string]) string { return k.key() }

func Show(o Open) string { return o.Label() }

func NewRef(id string) Ref { return Ref{id: id} }

func NewTagged[T any](id string) Tagged[T] { return Tagged[T]{id: id} }

// Read is why NewTagged hands back the concrete handle rather than Input.
func Read[T any](t Tagged[T]) string { return t.id }
