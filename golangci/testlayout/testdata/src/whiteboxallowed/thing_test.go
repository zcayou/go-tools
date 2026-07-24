package whiteboxallowed

import "testing"

func TestThing(t *testing.T) {
	if thing() != 1 {
		t.Fatal("thing")
	}
}
