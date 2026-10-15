package fixturenohelpers_test

import "testing"

func TestB(t *testing.T) {
	if shared() != 1 {
		t.Fatal("b")
	}
}
