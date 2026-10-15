package kinds

type Kind int

const (
	KindA Kind = iota + 1
	KindB
)

// Alias re-exports a member.
const Alias = KindB

func (k Kind) String() string {
	switch k {
	case KindA:
		return "a"
	case KindB:
		return "b"
	default:
		return "unknown"
	}
}

type Mode string

const ModeRead Mode = "read"

type Level int

const (
	LevelNone Level = iota
	LevelHigh
)

func (l Level) String() string {
	switch l {
	case LevelNone:
		return "none"
	case LevelHigh:
		return "high"
	default:
		return "unknown"
	}
}

// Labeled embeds a Kind, promoting its String.
type Labeled struct {
	Kind
}

// second spells a member's value, which the declaring package may: it owns
// the numbering.
func second(k Kind) bool { return k == 2 }

var _ = second
