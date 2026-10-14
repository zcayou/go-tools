// Package inner is a dependency the indirect fixture reaches only through
// outer, whose exported declarations never mention it.
package inner

// Acquirer is an interface whose consumer classifies what an implementation
// returns.
type Acquirer interface{ Acquire() error }

// Classified is an interface base.
type Classified interface{ Classify() int }

// Count counts.
func Count(Acquirer) int { return 1 }
