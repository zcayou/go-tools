package kinds

type State int

const (
	Pending State = iota + 1
	Running
	Failed
)

func (s State) String() string {
	switch s {
	case Pending:
		return "pending"
	case Running:
		return "running"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

// Finished orders a member in the declaring package, which owns what its
// ordinals mean.
func (s State) Finished() bool { return s >= Failed }

// Worst orders two members in the declaring package.
func Worst(a, b State) State { return max(a, b) }

// Level is ordered: its ordinals rise with severity.
type Level int

const (
	Debug Level = iota + 1
	Info
)

func (l Level) String() string {
	switch l {
	case Debug:
		return "debug"
	case Info:
		return "info"
	default:
		return "unknown"
	}
}

// Spare is an enum nothing outside its package orders.
type Spare int

const (
	SpareA Spare = iota + 1
	SpareB
)

func (s Spare) String() string {
	switch s {
	case SpareA:
		return "spare-a"
	case SpareB:
		return "spare-b"
	default:
		return "unknown"
	}
}
