package foreign

// Weekday is an enum of another module.
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
