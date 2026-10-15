package fixturehelpers_test

import "testing"

func TestAlpha(t *testing.T) {
	if onlyAlpha()+both()+base()+(probe{}).read() != 4 {
		t.Fatal("alpha")
	}
}
