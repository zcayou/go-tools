package kinds

import (
	"cmp"
	"slices"
)

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

// Sorted orders by ordinal in the declaring package, which owns what its
// ordinals mean.
func Sorted(ks []Kind) []Kind {
	slices.Sort(ks)
	slices.SortFunc(ks, func(a, b Kind) int { return cmp.Compare(a, b) })
	return ks
}

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
