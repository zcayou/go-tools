// Package states declares the enums tokenforms's specs assert, and types
// written like enums that are none.
package states

import "strconv"

type State int // want State:"enum"

const (
	StatePending State = iota + 1
	StateRunning
)

func (s State) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateRunning:
		return "running"
	default:
		return "State(" + strconv.Itoa(int(s)) + ")"
	}
}

type Mode string // want Mode:"enum"

const (
	ModeFast Mode = "fast"
	ModeSlow Mode = "slow"
)

// Flags is a flag set: its values combine.
type Flags int

const (
	FlagA Flags = 1 << iota
	FlagB
)

func (f Flags) String() string { return "flags" }

// Size is a quantity: its constants are units.
type Size int64

const (
	KB Size = 1024
	MB      = 1024 * KB
)

func (s Size) String() string { return "size" }

// Label renders itself but declares no constants, so it is no enum.
type Label int

func (l Label) String() string { return "label" }
