package main

import (
	"errors"
	"fmt"
)

// ErrBase is the sentinel the wrapper carries.
var ErrBase = errors.New("base")

// WrapError is unwrapped only by errors.Is, which discovers Unwrap by assertion.
type WrapError struct{ inner error }

func (e *WrapError) Error() string { return "wrap: " + e.inner.Error() }
func (e *WrapError) Unwrap() error { return e.inner }

func main() {
	fmt.Println(errors.Is(&WrapError{inner: ErrBase}, ErrBase))
}
