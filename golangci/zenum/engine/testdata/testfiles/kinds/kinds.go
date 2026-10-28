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

// secret is the package's own state, which only its tests print.
type secret int

const (
	secretA secret = iota + 1
	secretB
)
