// Package owner declares the vocabulary the other packages rename.
package owner

// Mode is the type of the constants below.
type Mode uint8

const (
	// On is spelled by the program, through a copy of a copy.
	On Mode = iota + 1
	// Off is spelled only by a test, through a copy.
	Off
	// Idle is spelled by nothing but its copies.
	Idle
)

// Box is spelled only by a test, through a generic copy.
type Box[T any] struct{ Value T }

// Limit is converted rather than renamed.
const Limit = 8

// Current is what the package calls the thing now.
type Current struct{}

// Former is the name it used to have: a second name within one package, which
// is a declaration of its own.
type Former = Current
