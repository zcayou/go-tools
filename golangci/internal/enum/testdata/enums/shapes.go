package enums

// Flags combine, so a value no constant names is ordinary: its shifts make it
// no enum.
type Flags uint8

const (
	FlagA Flags = 1 << iota
	FlagB
)

// Bit is a flag set from one written shift.
type Bit int

const BitHigh Bit = 1 << 3

// Size is a quantity: KB multiplies another of its constants.
type Size int64

const (
	Byte Size = 1
	KB        = 1024 * Byte
)

// Counted counts by adding, which keeps it an enum.
type Counted int

const (
	CountedA Counted = 1
	CountedB         = CountedA + 1
)

// Bounded closes its iota run with a sentinel that sizes an array.
type Bounded int

const (
	BoundedA Bounded = iota + 1
	BoundedB
	boundedCount
)

var boundedNames [boundedCount]string

// Ordered closes its iota run with a sentinel an ordering compares against.
type Ordered int

const (
	OrderedA Ordered = iota + 1
	OrderedB
	orderedEnd
)

func (o Ordered) valid() bool { return o < orderedEnd }

// Published closes its iota run with an exported constant, which a reader
// outside the package can name, so it stays a member.
type Published int

const (
	PublishedA Published = iota + 1
	PublishedB
	PublishedCount
)

var publishedNames [PublishedCount]string

// Unbounded closes its iota run with a constant nothing bounds by.
type Unbounded int

const (
	UnboundedA Unbounded = iota + 1
	unboundedB
)

// Explicit bounds by a constant that writes its own value rather than
// repeating the run.
type Explicit int

const (
	ExplicitA   Explicit = 1
	explicitMax Explicit = 2
)

var explicitNames [explicitMax]string

// Generic is a generic type, whose instantiations are no enum.
type Generic[T any] int

const GenericA Generic[int] = 1
