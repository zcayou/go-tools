package b

// Paint restates a.Color.
type Paint int

const (
	PaintRed Paint = iota + 1
	PaintGreen
)

func (p Paint) String() string {
	switch p {
	case PaintRed:
		return "red"
	case PaintGreen:
		return "green"
	default:
		return "unknown"
	}
}

// Tone restates a subset of a.Shade.
type Tone int

const (
	ToneDark Tone = iota + 1
	ToneLight
)

func (t Tone) String() string {
	switch t {
	case ToneDark:
		return "dark"
	case ToneLight:
		return "light"
	default:
		return "unknown"
	}
}

// Measure restates a.Size and extends it.
type Measure int

const (
	MeasureSmall Measure = iota + 1
	MeasureMedium
	MeasureLarge
)

func (m Measure) String() string {
	switch m {
	case MeasureSmall:
		return "small"
	case MeasureMedium:
		return "medium"
	case MeasureLarge:
		return "large"
	default:
		return "unknown"
	}
}

// Lone shares a.Solo's single token.
type Lone int

const Alone Lone = 1

func (Lone) String() string { return "only" }

// State restates a.Status, and a related-enums entry may say why.
type State int

const (
	StateOpen State = iota + 1
	StateClosed
)

func (s State) String() string {
	switch s {
	case StateOpen:
		return "open"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// Weekday restates the foreign module's Weekday, which is no enum's
// business here.
type Weekday int

const (
	Monday Weekday = iota + 1
	Tuesday
)

func (d Weekday) String() string {
	switch d {
	case Monday:
		return "monday"
	case Tuesday:
		return "tuesday"
	default:
		return "unknown"
	}
}
