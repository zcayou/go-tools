// Package facade renames owner's vocabulary so a consumer needs one import.
package facade

import (
	"reexport/internal/impl"
	"reexport/owner"
)

type (
	Mode       = owner.Mode
	Box[T any] = owner.Box[T]
)

const (
	On   = owner.On
	Off  = owner.Off
	Idle = owner.Idle
)

// Worker renames a declaration the declared surface does not make, so it is
// the surface's own name for it.
type Worker = impl.Worker

// Capacity converts owner's untyped constant rather than renaming it.
const Capacity int = owner.Limit
