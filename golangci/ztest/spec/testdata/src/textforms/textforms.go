package textforms

import "errors"

var ErrBoom = errors.New("boom")

func Do() error { return ErrBoom }

func Pair() (int, error) { return 0, ErrBoom }

type Kind int

func (k Kind) String() string { return "running" }
