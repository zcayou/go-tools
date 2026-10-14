// Package kind declares an internal enum, whose String is a display
// string.
package kind

type Kind int

const (
	A Kind = iota + 1
	B
)

func (k Kind) String() string {
	switch k {
	case A:
		return "a"
	case B:
		return "b"
	}
	return "unknown"
}
