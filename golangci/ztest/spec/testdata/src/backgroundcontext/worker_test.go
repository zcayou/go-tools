package backgroundcontext_test

import (
	"context"
	"testing"
)

var shared = context.Background() // want `^background-context: context\.Background in a test; take the context the test framework hands this node \(t\.Context, b\.Context, f\.Context, SpecContext\), or context\.WithoutCancel of it for work that outlives the node$`

func TestDrain(t *testing.T) {
	ctx := context.TODO() // want `^background-context: context\.TODO in a test; `
	_ = ctx
	_ = shared
	_ = t.Context()
}

func TestDetached(t *testing.T) {
	detached := context.WithoutCancel(t.Context())
	_ = detached
}

func helper() context.Context {
	return context.Background() // want `^background-context: context\.Background in a test; `
}

func TestValue(t *testing.T) {
	fresh := context.Background // want `^background-context: context\.Background in a test; `
	_ = fresh
	_ = helper
}

func TestMain(m *testing.M) {
	_ = context.Background()
	m.Run()
}

func Example() {
	_ = context.Background()
}

func ExampleRun() {
	_ = context.TODO()
}

func Examples() {
	_ = context.Background() // want `^background-context: context\.Background in a test; `
}
