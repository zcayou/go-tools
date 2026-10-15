package testifytext

import "errors"

var ErrBoom = errors.New("boom")

func Do() error { return ErrBoom }
