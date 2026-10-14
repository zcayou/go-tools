package kinds

// Flags is a flag set.
type Flags uint8

const (
	FlagA Flags = 1 << iota
	FlagB
	FlagC
)

// Bytes is a quantity.
type Bytes int64

const (
	Byte Bytes = 1
	KB         = 1024 * Byte
)

// Kind closes its run with a count sentinel.
type Kind int

const (
	KindA Kind = iota + 1
	KindB
	numKinds
)

var kindNames = [numKinds]string{KindA: "kind-a", KindB: "kind-b"}

func (k Kind) String() string {
	if k <= 0 || k >= numKinds {
		return "invalid"
	}
	return kindNames[k]
}
