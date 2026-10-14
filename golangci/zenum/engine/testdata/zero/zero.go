package zero

import "fmt"

// Kind starts at zero.
type Kind int

const (
	KindNone Kind = iota
	KindSome
)

func (k Kind) String() string {
	switch k {
	case KindNone:
		return "kind-none"
	case KindSome:
		return "kind-some"
	default:
		return "unknown"
	}
}

// Policy starts at zero too, which is its documented default.
type Policy int

const (
	PolicyDefault Policy = iota
	PolicyStrict
)

func (p Policy) String() string {
	switch p {
	case PolicyDefault:
		return "policy-default"
	case PolicyStrict:
		return "policy-strict"
	default:
		return "unknown"
	}
}

// Mode is a string enum whose empty value is a member.
type Mode string

const (
	ModeUnset Mode = ""
	ModeSet   Mode = "set"
)

// Count starts above zero.
type Count int

const (
	CountOne Count = iota + 1
	CountTwo
)

func (c Count) String() string {
	switch c {
	case CountOne:
		return "count-one"
	case CountTwo:
		return "count-two"
	default:
		return "unknown"
	}
}

// phase starts at zero, and nothing outside the package meets it.
type phase int

const (
	phaseIdle phase = iota
	phaseBusy
)

func (p phase) String() string {
	switch p {
	case phaseIdle:
		return "phase-idle"
	case phaseBusy:
		return "phase-busy"
	default:
		return fmt.Sprintf("phase(%d)", p)
	}
}
