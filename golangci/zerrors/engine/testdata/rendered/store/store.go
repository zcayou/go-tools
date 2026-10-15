// Package store declares the identifiers the errors render.
package store

import (
	"fmt"
	"strings"
)

// Key is comparable, so a caller can carry it.
type Key struct{ Bucket, Name string }

func (k Key) String() string { return k.Bucket + "/" + k.Name }

// Ref is generic, rendered per instantiation.
type Ref[T comparable] struct{ ID T }

func (r Ref[T]) String() string { return fmt.Sprint(r.ID) }

// Path is not comparable.
type Path []string

func (p Path) String() string { return strings.Join(p, "/") }

// Kind is an enum, whose String is a member's name.
type Kind int

const (
	KindA Kind = iota + 1
	KindB
)

func (k Kind) String() string {
	switch k {
	case KindA:
		return "a"
	case KindB:
		return "b"
	}
	return "unknown"
}
