// Package facade originates nothing: each function forwards to owner, and
// the near misses below do something a call to the original would not.
package facade

import (
	"forwarders/internal/hidden"
	"forwarders/owner"
)

func Encode(id string) string { return owner.Encode(id) }

func Wrap[S any](id string) owner.Box[S] { return owner.Wrap[S](id) }

// Forward renames owner.Pass and leaves its type argument to inference.
func Forward[T any](value T) T { return owner.Pass(value) }

func Join(sep string, parts ...string) string { return owner.Join(sep, parts...) }

func Reset() { owner.Reset() }

func Orphan() int { return owner.Orphan() }

// Swap reorders its arguments.
func Swap(a, b string) string { return owner.Swap(b, a) }

// Extra takes a second statement.
func Extra() int {
	value := owner.Extra()
	return value
}

// Narrow narrows the constraint.
func Narrow[T comparable](value T) T { return owner.Narrow(value) }

// Local forwards within its own package.
func Local() int { return local() }

func local() int { return 4 }

// Hidden forwards to a package the surface leaves out.
func Hidden() int { return hidden.Value() }
