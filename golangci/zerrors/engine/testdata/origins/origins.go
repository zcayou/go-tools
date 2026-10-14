// Package origins returns an error of every origin unstructured-return sorts.
package origins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"example.com/framework"
)

// NotFound is a nameable error type.
type NotFound struct{ Name string }

func (e *NotFound) Error() string { return "not found: " + e.Name }

type notFound struct{}

func (notFound) Error() string { return "not found" }

// ErrGone is an exported sentinel of an API package.
var ErrGone = errors.New("gone")

func New() error { return errors.New("new") }

func Errorf() error { return fmt.Errorf("code %d", 1) }

func ErrorfWrappingRaw() error { return fmt.Errorf("wrapping: %w", io.EOF) }

func JoinRaw() error { return errors.Join(errors.New("a"), io.EOF) }

func Unexported() error { return notFound{} }

func ForeignCall() error { return os.Remove("x") }

func ForeignInterface(ctx context.Context) error { return ctx.Err() }

func ForeignVariable() error { return io.EOF }

func ForeignField(c *framework.Carrier) error { return c.Err }

func ForeignChannelField(c *framework.Carrier) error { return <-c.Errs }

func ForeignChannelCall() error { return <-framework.Results() }

func FrameworkCall() error { return framework.Do() }

func FrameworkVariable() error { return framework.ErrClosed }

func FrameworkInterface(s framework.Source) error { return s.Next() }

func FrameworkType() error { return &framework.Error{Code: 2} }

func Nameable() error { return &NotFound{Name: "x"} }

func Sentinel() error { return ErrGone }

func ErrorfWrapping() error { return fmt.Errorf("wrapping: %w", &NotFound{}) }

func ErrorfMixed() error { return fmt.Errorf("%w and %w", io.EOF, &NotFound{}) }

func JoinMixed() error { return errors.Join(io.EOF, &NotFound{}) }

func Parameter(err error) error { return err }

func FunctionValue(f func() error) error { return f() }

func WalkDir() error {
	return filepath.WalkDir(".", func(string, fs.DirEntry, error) error { return nil })
}

func ErrGroup() error {
	var g framework.Group
	g.Go(func() error { return nil })
	return g.Wait()
}

func Receive(ch chan error) error { return <-ch }

func Index(errs []error) error { return errs[0] }

func Lookup(errs map[string]error) error { return errs["x"] }

func Deref(err *error) error { return *err }

func Assert(v any) error { return v.(error) }

func Generic[E error](e E) error { return e }

func Format(format string) error { return fmt.Errorf(format) }

func Branches(n int) error {
	switch n {
	case 0:
		return nil
	case 1:
		return &NotFound{}
	}
	return io.ErrUnexpectedEOF
}

func Deferred() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panicked: %v", r)
		}
	}()
	return &NotFound{}
}
