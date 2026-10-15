package fixturehelpers_test

import "testing"

func TestBeta(t *testing.T) {
	if both()+viaRelay()+(probe{}).read() != 3 {
		t.Fatal("beta")
	}
}
