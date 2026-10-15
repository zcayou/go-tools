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

// Value encodes a member, which is the declaring package's to do.
func (k Kind) Value() int64 { return int64(k) }

// Other is a second enum.
type Other int

const (
	OtherA Other = iota + 1
	OtherB
)

func (o Other) String() string {
	switch o {
	case OtherA:
		return "other-a"
	case OtherB:
		return "other-b"
	default:
		return "unknown"
	}
}

// Name is a string enum, which laundered-conversion does not judge.
type Name string

const (
	NameA Name = "name-a"
	NameB Name = "name-b"
)

// Code is a named type that is no enum.
type Code int
