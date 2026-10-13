// shared is misplaced, but broken_test.go could consume anything, so nothing
// is said about it.
package fixtureunparsable

import "testing"

func TestThing(t *testing.T) {
	if thing()+shared() != 2 {
		t.Fatal("thing")
	}
}

func shared() int { return 1 }
