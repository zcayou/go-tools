package catalog

type Presence int

const (
	Online Presence = iota + 1
	Away
	Busy
	Hidden
	Offline
)

func (p Presence) String() string {
	switch p {
	case Online:
		return "online"
	case Away:
		return "away"
	case Busy:
		return "busy"
	case Hidden:
		return "hidden"
	case Offline:
		return "offline"
	default:
		return "unknown"
	}
}
