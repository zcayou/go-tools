package kinds

type Kind int

const (
	KindNone Kind = iota
	KindSome
)

func (k Kind) String() string {
	switch k {
	case KindNone:
		return "none"
	case KindSome:
		return "some"
	default:
		return "unknown"
	}
}
