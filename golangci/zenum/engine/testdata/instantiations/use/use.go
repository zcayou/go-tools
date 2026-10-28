package use

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"example.com/instantiations/kinds"
)

func Max[T cmp.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

type Heap[T cmp.Ordered] struct{ items []T }

func Narrow[T interface{ ~int | ~uint8 }](a, b T) bool { return a < b }

func uses(ks []kinds.Kind, m map[kinds.Kind]int, k kinds.Kind, ls []kinds.Level) Heap[kinds.Kind] {
	slices.Sort(ks)
	_ = slices.Sorted(maps.Keys(m))
	slices.SortFunc(ks, func(a, b kinds.Kind) int { return cmp.Compare(a, b) })
	_ = Max(k, kinds.KindA)
	_ = Narrow(k, k)
	slices.Sort(ls)
	_ = slices.Contains(ks, k)
	slices.SortFunc(ks, func(a, b kinds.Kind) int { return strings.Compare(a.String(), b.String()) })
	return Heap[kinds.Kind]{}
}
