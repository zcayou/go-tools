// Package lib exposes generic methods on a type that is not generic itself, and
// nothing in the module instantiates them.
package lib

// Set is exported and not generic, so Len roots without help.
type Set struct{}

func (Set) Len() int { return 0 }

// Rare's body calls one concrete function, the same call whatever a consumer
// would instantiate it with, so rareHelper is reachable.
func (Set) Rare[K comparable](k K) K {
	rareHelper()
	return k
}

func rareHelper() {}

// Opaque calls through a function value, which a walk cannot follow without
// an instantiation's type flow.
func (Set) Opaque[K any](build func() K) K { return build() }
