package whiteboxcustom

import "testing"

func TestThing(t *testing.T) {
	if doubled(thing()) != 2 {
		t.Fatal("thing")
	}
}
