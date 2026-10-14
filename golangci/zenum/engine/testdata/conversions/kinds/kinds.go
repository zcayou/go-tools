package kinds

type Kind int

const (
	KindA Kind = iota + 1
	KindB
)

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

// Parse makes a member from a number, which is the declaring package's to do.
func Parse(n int) (Kind, bool) {
	k := Kind(n)
	return k, k == KindA || k == KindB
}

// Take hands a member back.
func Take(k Kind) Kind { return k }

// Make converts to a type parameter, in the declaring package.
func Make[T ~int](n int) T { return T(n) }
