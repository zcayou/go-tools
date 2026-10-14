// Package indirect imports only outer, and so reaches inner only
// transitively.
package indirect

import (
	"errors"

	"example.com/outer"
)

// Lane implements inner.Acquirer without naming it.
type Lane struct{}

func (Lane) Acquire() error { return errors.New("busy") }

// Classified implements inner.Classified without naming it.
type Classified struct{}

func (Classified) Error() string { return "classified" }

func (Classified) Classify() int { return outer.Run() }

// Plain does not.
type Plain struct{}

func (Plain) Error() string { return "plain" }
