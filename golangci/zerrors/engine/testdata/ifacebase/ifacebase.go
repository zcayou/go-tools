// Package ifacebase declares error types against an interface base.
package ifacebase

import "example.com/framework"

type Good struct{}

func (Good) Error() string { return "good" }

func (Good) Classify() int { return 1 }

type Pointer struct{}

func (*Pointer) Error() string { return "pointer" }

func (*Pointer) Classify() int { return 2 }

type Bad struct{}

func (Bad) Error() string { return "bad" }

var _ framework.Classified = Good{}

// Generic declares Classify with a signature the base does not.
type Generic[T any] struct{}

func (Generic[T]) Error() string { return "generic" }

func (Generic[T]) Classify() string { return "wrong" }
