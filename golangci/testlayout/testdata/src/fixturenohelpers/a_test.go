package fixturenohelpers_test

import "testing"

func TestA(t *testing.T) {
	if shared() != 1 {
		t.Fatal("a")
	}
}

func shared() int { return 1 } // want `misplaced-fixture: shared is used by a_test\.go, b_test\.go; blackbox\.helper-patterns is empty, so no file of this kind may hold a fixture another file uses`
