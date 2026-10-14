package conversions

import "example.com/conversions/kinds"

// Special is a constant of the enum's type declared outside its package,
// which is judged here rather than where it is used.
const Special kinds.Kind = 99

const raw = 6

var current kinds.Kind = 42

func implicit(k kinds.Kind, m map[kinds.Kind]string) []kinds.Kind {
	k = 7
	_ = m[4]
	_ = k < 3
	return []kinds.Kind{
		kinds.Take(8),
		kinds.KindB + 5,
		kinds.Take(raw),
		k,
		Special,
		kinds.Take(1),
		kinds.Take(0),
		kinds.KindA + 1,
		current,
	}
}

func cast[T ~int](n int) T { return T(n) }

func instantiate(n int) (kinds.Kind, kinds.Kind) {
	return cast[kinds.Kind](n), kinds.Make[kinds.Kind](n)
}
