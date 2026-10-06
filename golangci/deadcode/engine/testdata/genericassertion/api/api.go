// Package api boxes generic handles and hands them back through a generic
// assertion, so the only evidence that a handle's method runs is an interface
// the assertion is instantiated with.
package api

// Store holds values boxed in any.
type Store struct{ entries map[string]any }

// NewStore returns an empty Store.
func NewStore() *Store { return &Store{entries: map[string]any{}} }

// Bind returns the value stored under name as an N.
func Bind[N any](s *Store, name string) (N, bool) {
	n, ok := s.entries[name].(N)
	return n, ok
}

// Handle is generic, and Get returns its own parameter, so Handle as written
// matches no interface over a concrete Get.
type Handle[V any] struct{ value V }

func (h Handle[V]) Get() V { return h.value }

// Put stores value behind a Handle. Only the test calls it, and binds the result
// through an interface of its own.
func Put[V any](s *Store, name string, value V) { s.entries[name] = Handle[V]{value: value} }

// Slot is Handle's production twin: Total stores one and binds it through
// valuer.
type Slot[V any] struct{ value V }

func (s Slot[V]) Value() V { return s.value }

type valuer interface{ Value() int }

// Total stores a Slot and reads it back through valuer.
func Total(s *Store) int {
	s.entries["total"] = Slot[int]{value: 3}
	v, _ := Bind[valuer](s, "total")
	return v.Value()
}
