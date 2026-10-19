// Package deferred returns errors through closures deferred to define or amend
// a named result, as an attributing adapter and a close-join do.
package deferred

import (
	"errors"
	"fmt"
	"os"
)

// Error attributes an operation's failure.
type Error struct {
	Operation string
	Cause     error
}

func (e *Error) Error() string { return e.Operation }

func (e *Error) Unwrap() error { return e.Cause }

// NotFound is a nameable error type.
type NotFound struct{}

func (*NotFound) Error() string { return "not found" }

func attribute(operation string, cause error) error {
	if cause == nil {
		return nil
	}
	return &Error{Operation: operation, Cause: cause}
}

func Attributed(ok bool) (err error) {
	defer func() { err = attribute("push", err) }()
	if !ok {
		return errors.New("refused")
	}
	return nil
}

func AttributedWhen(attributing bool) (err error) {
	if attributing {
		defer func() { err = attribute("push", err) }()
	}
	return errors.New("refused")
}

func AttributedIf(attributing bool) (err error) {
	defer func() {
		if attributing {
			err = attribute("push", err)
		}
	}()
	return errors.New("refused")
}

func Copied(name string) error { return copied(name) }

func copied(name string) (err error) {
	file, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("%w: open %q: %w", &NotFound{}, name, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("%w: close %q: %w", &NotFound{}, name, closeErr))
		}
	}()
	_, err = file.Stat()
	if err != nil {
		return fmt.Errorf("%w: stat %q: %w", &NotFound{}, name, err)
	}
	return nil
}

func Closed(name string) error { return closed(name) }

func closed(name string) (err error) {
	file, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("%w: open %q: %w", &NotFound{}, name, err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	return nil
}

func Recovered(name string) error { return recovered(name) }

func recovered(name string) (err error) {
	defer func() {
		if recover() != nil {
			err = &NotFound{}
		}
	}()
	_, err = os.Open(name)
	if err != nil {
		return fmt.Errorf("%w: open %q: %w", &NotFound{}, name, err)
	}
	return nil
}
