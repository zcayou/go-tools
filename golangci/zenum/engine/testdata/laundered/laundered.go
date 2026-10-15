package laundered

import (
	"fmt"
	"strconv"
	"sync/atomic"

	"example.com/laundered/kinds"
)

var counts [3]int

func launder(k kinds.Kind, n kinds.Name) []any {
	var stored atomic.Int64
	var raw int32
	stored.Store(int64(k))
	stored.Add((int64(k)))
	atomic.StoreInt32(&raw, int32(k))
	return []any{
		counts[int(k)],
		strconv.Itoa(int(k)),
		int64(kinds.KindB),
		kinds.Code(k),
		kinds.Other(k),
		kinds.Kind(k),
		any(k),
		fmt.Stringer(k),
		string(n),
		widen[int](k),
		widen[kinds.Kind](k),
		number(k),
		number(7),
	}
}

func widen[T ~int](k kinds.Kind) T { return T(k) }

func number[E ~int](e E) int { return int(e) }

func store[E ~int64](v *atomic.Int64, e E) { v.Store(int64(e)) }

var _ = store[int64]
