// Package store declares error types that carry the base, and ones that do not.
package store

import "example.com/base/contract"

type NotFound struct{ contract.Classification }

type middle struct{ contract.Classification }

type Chained struct{ middle }

type Pointer struct{ *contract.Classification }

func (*Pointer) Error() string { return "pointer" }

type Missing struct{}

func (*Missing) Error() string { return "missing" }

type classified struct{ contract.Classification }

type lost struct{}

func (lost) Error() string { return "lost" }

func Find() error { return &classified{} }

func Lose() error { return lost{} }
