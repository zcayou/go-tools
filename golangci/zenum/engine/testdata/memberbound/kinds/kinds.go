package kinds

type Kind int

const (
	KindA Kind = iota + 1
	KindB
	KindC
)

func (k Kind) String() string {
	switch k {
	case KindA:
		return "a"
	case KindB:
		return "b"
	case KindC:
		return "c"
	default:
		return "unknown"
	}
}

// Valid bounds Kind by its highest member, and by its first, which is clean.
func (k Kind) Valid() bool { return k >= KindA && k <= KindC }

// Invalid checks the range from outside it.
func (k Kind) Invalid() bool { return k < KindA || k > KindC }

// Last bounds through max.
func Last(k Kind) Kind { return max(k, KindC) }

// Sized ends in a count sentinel, which bounds it as the members grow.
type Sized int

const (
	SizedA Sized = iota + 1
	SizedB
	sizedCount
)

func (s Sized) String() string {
	switch s {
	case SizedA:
		return "small"
	case SizedB:
		return "large"
	default:
		return "unknown"
	}
}

func (s Sized) Valid() bool { return s >= SizedA && s < sizedCount }

// Level is ordered: a comparison with its highest member is a threshold.
type Level int

const (
	Debug Level = iota + 1
	Info
	Error
)

func (l Level) String() string {
	switch l {
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Error:
		return "error"
	default:
		return "unknown"
	}
}

func (l Level) Severe() bool { return l >= Error }

// Shape says which values are members by switching over them.
type Shape int

const (
	Circle Shape = iota + 1
	Square
)

func (s Shape) String() string {
	switch s {
	case Circle:
		return "circle"
	case Square:
		return "square"
	default:
		return "unknown"
	}
}

func (s Shape) Valid() bool {
	switch s {
	case Circle, Square:
		return true
	}
	return false
}
