// Package owner declares the functions facade forwards to.
package owner

func Encode(id string) string { return "id:" + id }

type Box[S any] struct{ id string }

func Wrap[S any](id string) Box[S] { return Box[S]{id: id} }

func Pass[T any](value T) T { return value }

func Join(sep string, parts ...string) string {
	out := ""
	for _, part := range parts {
		out += sep + part
	}
	return out
}

// Reset is reached only by the test, through facade.
func Reset() {}

// Orphan is forwarded and referenced by nothing else, so it is reported here,
// once.
func Orphan() int { return 1 }

func Swap(a, b string) string { return a + b }

func Extra() int { return 2 }

func Narrow[T any](value T) T { return value }
