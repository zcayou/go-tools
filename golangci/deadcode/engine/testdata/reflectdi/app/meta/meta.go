// Package meta holds a product whose metadata reaches the derived-type
// closure only as a field: no built body ever converts a Metadata to any
// interface.
package meta

import "fmt"

// Defaults is the datum appliers receive.
type Defaults struct{ Limit int }

// Spec is the registered product; its field carries Metadata into the
// closure.
type Spec struct {
	Meta Metadata
}

func NewSpec() *Spec { return &Spec{} }

// Metadata's Apply is demanded by nothing but the container's generic-body
// assertion.
type Metadata struct {
	floor int
}

func (m Metadata) Apply(d Defaults) error {
	if d.Limit < m.floor {
		return fmt.Errorf("limit %d below floor %d", d.Limit, m.floor)
	}
	return nil
}

// Mismatch is the control: an identically named Apply whose signature the
// asserted interface refuses, on a type the program does materialize.
type Mismatch struct{}

func NewMismatch() *Mismatch { return &Mismatch{} }

func (Mismatch) Apply(wrong string) error {
	return fmt.Errorf("nothing applies %q", wrong)
}
