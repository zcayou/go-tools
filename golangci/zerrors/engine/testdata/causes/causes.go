// Package causes holds an error type whose cause slot one value fills with
// a raw error and another with a nameable one.
package causes

import (
	"errors"
	"fmt"
)

// Error carries a cause.
type Error struct{ cause error }

func (e *Error) Error() string { return "failed" }

func (e *Error) Unwrap() error { return e.cause }

// NotFound is a nameable error type.
type NotFound struct{}

func (*NotFound) Error() string { return "not found" }

func encode() error { return &Error{cause: fmt.Errorf("bad encoding")} }

func Lookup() *Error { return &Error{cause: &NotFound{}} }

func Cause(e *Error) error { return e.cause }

func Rewrapped(e *Error) error { return fmt.Errorf("again: %w", e.cause) }

func Joined(e *Error) error { return errors.Join(e.cause, fmt.Errorf("closing")) }

func release(e *Error, held bool) error {
	switch {
	case e == nil:
		return nil
	case held:
		return &NotFound{}
	}
	return e.cause
}

func joinFailure(existing, failure error) error { return errors.Join(existing, failure) }

func Released(e *Error) error { return joinFailure(release(e, true), release(e, false)) }

var _ = encode
