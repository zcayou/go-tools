//go:build go1.1
// +build go1.1
// the constraint above is not prose

package directives

// A legacy constraint reads as an ordinary sentence and is not one. Joined
// to the line under it, go vet reports that the two forms no longer agree
// and gofmt writes it back out.

// want +2 `comment-wrap: .* over 2 lines; .* takes 1`

// prose still
// fills here
type Constrained struct{}
