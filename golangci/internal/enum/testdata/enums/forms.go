package enums

import (
	"fmt"
	"time"
)

// Switch names its members in a switch on the receiver.
type Switch int

const (
	SwitchA Switch = iota + 1
	SwitchB
	SwitchC
)

// SwitchAlias re-exports a member under another name.
const SwitchAlias = SwitchB

func (s Switch) String() string {
	switch s {
	case SwitchA:
		return "a"
	case SwitchB:
		return "b"
	case SwitchC:
		return "c"
	default:
		return "unknown"
	}
}

// Table indexes a table of names, behind a guard that sends what the table has
// no entry for elsewhere.
type Table int

const (
	TableFirst Table = iota + 1
	TableSecond
)

var tableNames = [...]string{"", "first", "second"}

func (t Table) String() string {
	if t <= 0 || int(t) >= len(tableNames) {
		return "Table(" + fmt.Sprint(int(t)) + ")"
	}
	return tableNames[t]
}

// Offset indexes a slice of names by an offset receiver, inside a guard that
// sends what the table holds into it.
type Offset uint8

const (
	OffsetFirst Offset = iota + 1
	OffsetSecond
)

var offsetNames = []string{"first", "second"}

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
	MappedOne: "one",
	MappedTwo: "two",
}

func (m Mapped) String() string {
	return mappedNames[m]
}

// Ambiguous leaves one member to the code after its switch, and gives two
// others one name.
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

// Ratio is neither ordinal nor textual.
type Ratio float64

const RatioHalf Ratio = 0.5

// Timeout is a constant of a foreign type, which makes no enum here.
const Timeout = 3 * time.Second

// Loose has no constant at all.
type Loose int
