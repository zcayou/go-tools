package booleanidentity

import "errors"

var (
	ErrMissing = errors.New("missing")
	ErrClosed  = errors.New("closed")
)

type CodeError struct {
	Code int
}

func (e *CodeError) Error() string { return "code" }

func Do() error { return ErrMissing }
