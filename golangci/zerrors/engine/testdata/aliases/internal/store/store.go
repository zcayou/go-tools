// Package store is internal, and so never an API package.
package store

import "errors"

type NotFound struct{}

func (*NotFound) Error() string { return "not found" }

type Hidden struct{}

func (*Hidden) Error() string { return "hidden" }

type Store struct{}

func (*Store) Get() error { return errors.New("unavailable") }
