// Package followed returns errors whose origins unstructured-return follows
// through module code.
package followed

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// NotFound is a nameable error type.
type NotFound struct{}

func (*NotFound) Error() string { return "not found" }

func Open() error { return readManifest() }

func readManifest() error { return fmt.Errorf("bad manifest") }

func Wrapped() error { return pass(io.EOF) }

func Clean() error { return pass(&NotFound{}) }

func pass(err error) error { return err }

type store interface{ get() error }

type memory struct{}

func (memory) get() error { return errors.New("evicted") }

type disk struct{}

func (disk) get() error { return &NotFound{} }

func Get(cached bool) error {
	var s store = disk{}
	if cached {
		s = memory{}
	}
	return s.get()
}

type closer interface{ Close() error }

func Close(f *os.File) error {
	var c closer = f
	return c.Close()
}

// Store is an open interface: consumers implement it.
type Store interface{ Load() error }

func Fetch(s Store) error { return s.Load() }

type result struct{ err error }

func run() result { return result{err: os.Remove("x")} }

func Run() error {
	r := run()
	return r.err
}

type outcome struct{ err error }

func settle() *outcome { return &outcome{err: &NotFound{}} }

func Settle() error { return settle().err }

func Recursive(n int) error {
	if n == 0 {
		return countdown(n)
	}
	return Recursive(n - 1)
}

func countdown(n int) error {
	if n > 0 {
		return countdown(n - 1)
	}
	return errors.New("liftoff")
}

func Immediate() error {
	err := io.EOF
	return func() error { return err }()
}
