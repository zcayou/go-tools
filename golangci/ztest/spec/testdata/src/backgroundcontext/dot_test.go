package backgroundcontext_test

import (
	. "context"
	"testing"
)

// Background here is the dot-imported one, and TODO a method of a local type
// that merely shares the name.
type todos struct{}

func (todos) TODO() int { return 0 }

func TestDot(t *testing.T) {
	_ = Background() // want `^background-context: context\.Background in a test; `
	_ = todos{}.TODO()
	_ = WithoutCancel(t.Context())
}
