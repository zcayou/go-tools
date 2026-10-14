// Package flow returns errors through closures, range-over-func loops,
// receivers selected from fields, and wrappings of parameters.
package flow

import (
	"errors"
	"fmt"
	"io"
	"iter"

	"example.com/framework"
)

// NotFound is a nameable error type.
type NotFound struct{}

func (*NotFound) Error() string { return "not found" }

func capture(err error) error { return func() error { return err }() }

func CaptureRaw() error { return capture(io.EOF) }

func CaptureNameable() error { return capture(&NotFound{}) }

func Nested() error {
	err := io.ErrUnexpectedEOF
	return func() error { return func() error { return err }() }()
}

func Ranged(seq iter.Seq[int]) error {
	for v := range seq {
		if v == 3 {
			return errors.New("three")
		}
	}
	return nil
}

// Service runs callbacks through a group it holds.
type Service struct{ group framework.Group }

func (s *Service) Run() error {
	s.group.Go(func() error { return nil })
	return s.group.Wait()
}

func JoinParameter(err error) error { return errors.Join(err, io.EOF) }

func closeAll(err error, c io.Closer) error { return errors.Join(err, c.Close()) }

func Save(c io.Closer) error { return closeAll(&NotFound{}, c) }

func annotate(err error) error { return fmt.Errorf("%w: %w", io.EOF, err) }

func Annotated() error { return annotate(&NotFound{}) }

func AnnotatedRaw() error { return annotate(io.ErrClosedPipe) }

func Cleanup(c io.Closer) (err error) {
	defer func() { err = errors.Join(err, c.Close()) }()
	return nil
}

func JoinNil() error { return errors.Join(nil, io.EOF) }

func check(ok bool) error {
	if ok {
		return nil
	}
	return &NotFound{}
}

func Checked(ok bool) error {
	if err := check(ok); err != nil {
		return fmt.Errorf("checking: %w", err)
	}
	return nil
}
