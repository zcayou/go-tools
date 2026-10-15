package ordered

import (
	"slices"

	"example.com/ordered/kinds"
)

func order(s, t kinds.State, l kinds.Level, states []kinds.State) []any {
	for x := kinds.Pending; x <= kinds.Failed; x++ {
		_ = x
	}
	slices.Sort(states)
	return []any{
		s > kinds.Running,
		kinds.Running < s,
		s >= t,
		max(s, t),
		min(s, kinds.Running),
		s == t,
		s.Finished(),
		l > kinds.Debug,
		less(s, t),
	}
}

func less[E ~int](a, b E) bool { return a < b }
