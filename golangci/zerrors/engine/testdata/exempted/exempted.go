// Package exempted hands the framework an exported type through one of its
// interfaces.
package exempted

import (
	"errors"

	"example.com/framework"
)

// Lane is an acquirer the framework is handed.
type Lane struct{}

func (Lane) Acquire() error { return errors.New("busy") }

func Install() { framework.Use(Lane{}) }
