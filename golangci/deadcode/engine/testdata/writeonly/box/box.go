// Package box converts values to an interface to call their methods.
package box

import "io"

type state struct{ outcome int }

type owner struct{ st *state }

func (o *owner) Close() error { return nil }

type unboxedState struct{ outcome int }

type unboxedOwner struct{ st *unboxedState }

func (o *unboxedOwner) Close() error { return nil }

func Use() error {
	o := &owner{st: &state{}}
	o.st.outcome = 1
	var c io.Closer = o
	u := &unboxedOwner{st: &unboxedState{}}
	u.st.outcome = 1
	_ = u.Close()
	return c.Close()
}

type escapedState struct{ outcome int }

type escapedOwner struct{ st *escapedState }

func (o *escapedOwner) Close() error { return nil }

// Escape hands the interface on, so whatever receives it may reflect on it.
func Escape() io.Closer {
	e := &escapedOwner{st: &escapedState{}}
	e.st.outcome = 1
	var c io.Closer = e
	_ = c.Close()
	return c
}
