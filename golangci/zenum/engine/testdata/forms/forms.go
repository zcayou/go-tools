package forms

import "fmt"

// Switch names its members in a switch on the receiver.
type Switch int

const (
	SwitchA Switch = iota + 1
	SwitchB
	SwitchC
)

func (s Switch) String() string {
	switch s {
	case SwitchA:
		return "switch-a"
	case SwitchB:
		return "switch-b"
	case SwitchC:
		return "switch-c"
	default:
		return "unknown"
	}
}

// Table indexes a guarded table.
type Table int

const (
	TableFirst Table = iota + 1
	TableSecond
)

var tableNames = [...]string{"", "table-first", "table-second"}

func (t Table) String() string {
	if t <= 0 || int(t) >= len(tableNames) {
		return "Table(" + fmt.Sprint(int(t)) + ")"
	}
	return tableNames[t]
}

// Offset indexes a slice at an offset, inside a guard.
type Offset uint8

const (
	OffsetFirst Offset = iota + 1
	OffsetSecond
)

var offsetNames = []string{"offset-first", "offset-second"}

func (o Offset) String() string {
	if i := int(o) - 1; i < len(offsetNames) {
		return offsetNames[i]
	}
	return "unknown"
}

// Mapped looks its names up in a map keyed by member.
type Mapped int

const (
	MappedOne Mapped = iota + 1
	MappedTwo
)

var mappedNames = map[Mapped]string{
	MappedOne: "mapped-one",
	MappedTwo: "mapped-two",
}

func (m Mapped) String() string {
	return mappedNames[m]
}

// Ambiguous gives two members one token, one an empty token, and leaves one
// to the fallback.
type Ambiguous int

const (
	AmbiguousA Ambiguous = iota + 1
	AmbiguousB
	AmbiguousC
	AmbiguousD
)

func (a Ambiguous) String() string {
	switch a {
	case AmbiguousA, AmbiguousB:
		return "same"
	case AmbiguousC:
		return ""
	}
	return "other"
}

// Formatted renders by formatting, which no readable form names.
type Formatted int

const FormattedOne Formatted = 1

func (f Formatted) String() string {
	return fmt.Sprintf("formatted-%d", int(f))
}

// quiet renders unreadably too, but nothing outside the package meets it.
type quiet int

const quietOne quiet = 1

func (q quiet) String() string {
	return fmt.Sprintf("quiet-%d", int(q))
}

// Nameless declares no name.
type Nameless int

const NamelessOne Nameless = 1

// Pointer names itself only through a pointer receiver, which is no name
// a value has.
type Pointer int

const PointerOne Pointer = 1

func (p *Pointer) String() string { return "pointer" }

// Mode is a string enum.
type Mode string

const (
	ModeRead  Mode = "read"
	ModeWrite Mode = "write"
)
