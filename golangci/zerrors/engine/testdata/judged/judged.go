// Package judged holds functions unstructured-return judges, and ones it does
// not.
package judged

import (
	"errors"
	"io"

	"example.com/framework"
	"example.com/judged/internal/hidden"
)

// Fetcher is an exported interface consumers call.
type Fetcher interface{ Fetch() error }

type fetcher struct{}

func (fetcher) Fetch() error { return errors.New("unreachable") }

func NewFetcher() Fetcher { return fetcher{} }

type handler struct{}

func (handler) Handle() error { return errors.New("unhandled") }

type lane struct{}

func (lane) Acquire() error { return errors.New("busy") }

func Install() {
	framework.Register(handler{})
	framework.Use(lane{})
}

// Buffer answers to io.Writer's contract, which expects io.ErrShortWrite.
type Buffer struct{}

var _ io.Writer = (*Buffer)(nil)

func (*Buffer) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

// Wrapped is an error type with a cause.
type Wrapped struct{ cause error }

func (w *Wrapped) Error() string { return "wrapped" }

func (w *Wrapped) Unwrap() error { return w.cause }

func Wrap() error { return &Wrapped{cause: io.EOF} }

// ErrGone is an exported sentinel.
var ErrGone = errors.New("gone")

var errHidden = errors.New("hidden")

func Gone() error { return ErrGone }

func Hidden() error { return errHidden }

func Produce() error { return errors.New("raw") }

func Relay() error { return Produce() }

func RelayThroughHelper() error { return relay() }

func relay() error { return Produce() }

func Mixed(produced bool) error {
	if produced {
		return Produce()
	}
	return io.EOF
}

func Internal() error { return hidden.Raw() }
