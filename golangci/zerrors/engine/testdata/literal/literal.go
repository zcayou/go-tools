// Package literal stores into the string fields of error types.
package literal

import (
	"encoding/json"
	"fmt"
	"os"

	"example.com/literal/internal/kind"
)

// Kind is an exported enum of an API package.
type Kind int

const (
	KindA Kind = iota + 1
	KindB
)

func (k Kind) String() string {
	switch k {
	case KindA:
		return "a"
	case KindB:
		return "b"
	}
	return "unknown"
}

// Reason is a named string type.
type Reason string

type Literal struct{ Reason string }

func (e *Literal) Error() string { return e.Reason }

type Param struct{ Reason string }

func (e *Param) Error() string { return e.Reason }

type Carried struct{ Reason string }

func (e *Carried) Error() string { return e.Reason }

type Named struct{ Reason string }

func (e *Named) Error() string { return e.Reason }

type Tags struct{ Tags []string }

func (e *Tags) Error() string { return "tags" }

type Exported struct{ Reason string }

func (e *Exported) Error() string { return e.Reason }

type Display struct{ Reason string }

func (e *Display) Error() string { return e.Reason }

type Dynamic struct{ Reason string }

func (e *Dynamic) Error() string { return e.Reason }

type Typed struct{ Reason Reason }

func (e *Typed) Error() string { return string(e.Reason) }

type Never struct{ Reason string }

func (e *Never) Error() string { return e.Reason }

func raiseLiteral(b bool) error {
	if b {
		return &Literal{Reason: "a"}
	}
	return &Literal{Reason: "b" + "c"}
}

func raiseParam(b bool) error {
	if b {
		return newParam("x")
	}
	return newParam("y")
}

func newParam(reason string) error { return &Param{Reason: reason} }

type carrier struct{ reason string }

func raiseCarried() error {
	c := carrier{reason: "x"}
	return &Carried{Reason: c.reason}
}

func raiseNamed(k Kind) error { return &Named{Reason: k.String()} }

func raiseTags() error { return &Tags{Tags: []string{"a", "b"}} }

func NewExported(reason string) error { return &Exported{Reason: reason} }

func raiseExported() error { return NewExported("x") }

func raiseDisplay(k kind.Kind) error { return &Display{Reason: k.String()} }

func raiseDynamic(name string) error {
	if name == "" {
		return &Dynamic{Reason: "empty"}
	}
	return &Dynamic{Reason: os.Getenv(name)}
}

func raiseTyped() error { return &Typed{Reason: "x"} }

var _ = []any{raiseLiteral, raiseParam, raiseCarried, raiseNamed, raiseTags, raiseExported, raiseDisplay,
	raiseDynamic, raiseTyped}

// Spec is consumer input: consumers store into its exported fields, so
// the module's own stores are not all of them.
type Spec struct{ Image string }

type Input struct{ Image string }

func (e *Input) Error() string { return e.Image }

var defaultSpec = Spec{Image: "alpine"}

func raiseInput() error { return &Input{Image: defaultSpec.Image} }

var _ = raiseInput

// Decoded is filled by a decoder as well as by the module's own literal.
type Decoded struct {
	Code string `json:"code"`
}

func (e *Decoded) Error() string { return e.Code }

func raiseDecoded() error { return &Decoded{Code: "internal"} }

func decode(data []byte) error {
	var e Decoded
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return &e
}

// Converted is converted from a struct holding a value no constant gave it.
type Converted struct{ Reason string }

func (e *Converted) Error() string { return e.Reason }

type converting struct{ Reason string }

func raiseConverted(reason string) error {
	c := Converted(converting{Reason: reason})
	return &c
}

func raiseConvertedLiteral() error { return &Converted{Reason: "x"} }

var _ = []any{raiseDecoded, decode, raiseConverted, raiseConvertedLiteral}

// Handled is fed from a driver handed out as any, to code that cannot write
// its unexported fields.
type Handled struct{ Target string }

func (e *Handled) Error() string { return e.Target }

type driver struct{ id string }

func Driver() (any, error) { return &driver{id: "container"}, nil }

func (d *driver) fail() error { return &Handled{Target: d.id} }

// Fault is fed in the body of a generic type nothing instantiates.
type Fault struct{ Control string }

func (e *Fault) Error() string { return e.Control }

type ref[C any] struct{ control C }

func (r ref[C]) String() string { return fmt.Sprint(r.control) }

func (r ref[C]) fault() error { return &Fault{Control: r.String()} }

func raiseFault() error { return &Fault{Control: "fixed"} }

// Op is fed through a parameter a generic caller nothing instantiates passes
// a value its type argument decides.
type Op struct{ Name string }

func (e *Op) Error() string { return e.Name }

func newOp(name string) error { return &Op{Name: name} }

func raiseOp() error { return newOp("read") }

func opOf[T fmt.Stringer](v T) error { return newOp(v.String()) }

var _ = []any{(*driver).fail, raiseFault, raiseOp}

// Partial is set to one constant, but one construction leaves it unset, so it
// holds that constant or nothing.
type Partial struct {
	Reason string
	Code   int
}

func (e *Partial) Error() string { return e.Reason }

func raisePartial(b bool) error {
	if b {
		return &Partial{Reason: "x"}
	}
	return &Partial{Code: 1}
}

var _ = raisePartial
