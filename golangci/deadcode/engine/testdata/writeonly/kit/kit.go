// Package kit is what tests drive the store with.
package kit

import "writeonly/store"

func Depth(p *store.Probe) int { return p.Depth }

// Harness is read by its own package.
type Harness struct{ runs int }

func (h *Harness) Run() int {
	h.runs++
	return h.runs
}

// Fixture is what tests elsewhere build and read.
type Fixture struct{ X int }

func NewFixture() Fixture { return Fixture{X: 1} }
