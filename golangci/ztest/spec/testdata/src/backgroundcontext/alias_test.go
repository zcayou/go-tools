package backgroundcontext_test

import (
	"testing"

	stdctx "context"
)

func TestAlias(t *testing.T) {
	_ = stdctx.Background() // want `^background-context: context\.Background in a test; `
}
