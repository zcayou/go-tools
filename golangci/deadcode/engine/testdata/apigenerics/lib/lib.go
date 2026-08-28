// Package lib is a library whose way in is generic: the module holds no
// program, so the only concrete instantiations of its public surface are
// the ones its own test writes.
package lib

// Store is the generic public type a consumer instantiates.
type Store[V any] struct{ values []V }

// Put is an exported method of the generic type, and retain's only caller.
func (s *Store[V]) Put(value V) {
	s.values = append(s.values, value)
	retain(len(s.values))
}

// Load is an exported generic function, and prepare's only caller. It boxes
// a type-parameter value, which is what makes the partial instantiation Merge
// records one RTA cannot be handed.
func Load[V any](values []V) *Store[V] {
	prepare()
	observe(values)
	return &Store[V]{values: values}
}

// Merge calls Load from inside its own generic body, so the program carries
// a Load instantiated with a type parameter rather than a type.
func Merge[V any](left, right []V) *Store[V] {
	return Load(append(left, right...))
}

// Describe is exported and not generic, so it roots without help.
func Describe() string { return describeHelper() }

func retain(count int) { _ = count }

func prepare() {}

func observe(value any) { _ = value }

func describeHelper() string { return "store" }

// auditHelper is production code no instantiation of the public surface
// reaches, and the test calls it directly. Rooting instantiations must leave it
// reported: standing in for a consumer says nothing about a declaration
// no consumer can arrive at.
func auditHelper() string { return "audited" }

// Rare is exported and generic, and nothing in the module instantiates it. Its
// body calls one concrete function, which is the same call whatever a consumer
// would instantiate it with, so rareHelper is reachable and the analysis says so.
func Rare[V any](values []V) string {
	_ = values
	return rareHelper()
}

func rareHelper() string { return "rare" }

// Opaque is also never instantiated, and the only call in its body goes through
// a function value. Following that needs the type flow an instantiation would
// carry, so the analysis reports the gap rather than what lies past it.
func Opaque[V any](apply func(V) string, value V) string { return apply(value) }

// orphan is reached by nothing, generic surface or otherwise.
func orphan() {}
