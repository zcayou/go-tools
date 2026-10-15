package polarity

import "errors"

var ErrBoom = errors.New("boom")

func Do() error { return ErrBoom }

func Pair() (int, error) { return 0, ErrBoom }

// Failure is an error type, whose pointer implements error.
type Failure struct{}

func (*Failure) Error() string { return "failure" }
