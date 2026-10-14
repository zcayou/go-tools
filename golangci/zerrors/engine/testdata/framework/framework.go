// Package framework is the foreign side of every zerrors fixture: a module
// the fixtures require through replace, and name as trusted or leave untrusted.
package framework

import "errors"

// ErrClosed is a sentinel.
var ErrClosed = errors.New("closed")

// Do returns an error of the framework's.
func Do() error { return &Error{Code: 1} }

// Error is the framework's error type.
type Error struct{ Code int }

func (e *Error) Error() string { return "framework error" }

// Code is neither a struct nor an interface.
type Code int

// Source is an interface module code calls.
type Source interface{ Next() error }

// Handler is an interface the framework calls module types through.
type Handler interface{ Handle() error }

// Register hands the framework a handler.
func Register(Handler) {}

// Acquirer is an interface whose consumer classifies whatever an
// implementation returns.
type Acquirer interface{ Acquire() error }

// Use hands the framework an acquirer.
func Use(Acquirer) {}

// Carrier carries an error, and a channel of them.
type Carrier struct {
	Err  error
	Errs chan error
}

// Results returns a channel of errors.
func Results() <-chan error { return nil }

// Group runs callbacks and returns the first error one returned, as
// errgroup.Group does.
type Group struct{ err error }

// Go runs f.
func (g *Group) Go(f func() error) { g.err = f() }

// Wait returns the first error a callback returned.
func (g *Group) Wait() error { return g.err }

// Classification is a struct base.
type Classification struct{ Class int }

// Classified is an interface base.
type Classified interface{ Classify() int }
