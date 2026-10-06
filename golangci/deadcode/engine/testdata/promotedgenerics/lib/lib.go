// Package lib declares its generic operations on an unexported type and
// promotes them onto the exported one a consumer holds, so the only way in to
// them is through Caller's method set.
package lib

// Caller is the exported handle. Every method a consumer calls on it is
// promoted from access.
type Caller struct{ access }

type access struct{}

// Fetch is generic, promoted onto Caller, and instantiated only by the test.
func (access) Fetch[T any]() T {
	fetchHelper()
	var zero T
	return zero
}

// Rare is generic, promoted onto Caller, and instantiated by nothing. Its body
// calls rareHelper whatever a consumer would instantiate it with.
func (access) Rare[T any]() T {
	rareHelper()
	var zero T
	return zero
}

// Opaque is promoted and never instantiated, and its only call goes through
// a function value the walk cannot follow.
func (access) Opaque[T any](build func() T) T { return build() }

// Plain is promoted and not generic, so it roots through Caller's method set
// as it stands.
func (access) Plain() { plainHelper() }

// Direct is a generic method declared on Caller itself.
func (Caller) Direct[T any]() { directHelper() }

// worker is unexported and embedded in nothing exported, so no consumer
// reaches Run however the test instantiates it.
type worker struct{}

func (worker) Run[T any]() { runHelper() }

func fetchHelper() {}

func rareHelper() {}

func plainHelper() {}

func directHelper() {}

func runHelper() {}
