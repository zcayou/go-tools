package user

import "example.com/testfiles/kinds"

// Default is the kind a user starts with.
func Default() kinds.Kind { return kinds.KindA }

// Convert converts to a type parameter, which only a test instantiates with
// an enum.
func Convert[T ~int](n int) T { return T(n) }

// Logger is a printer only where a test implements it as one.
type Logger interface {
	Logf(format string, args ...any)
}

// Report formats through a Logger no plain code implements.
func Report(l Logger, k kinds.Kind) { l.Logf("%d", k) }
