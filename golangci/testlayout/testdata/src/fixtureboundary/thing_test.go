package fixtureboundary_test

import (
	"testing"

	fb "fixtureboundary"
)

func TestThing(t *testing.T) {
	if fb.Thing()+fb.Built() != 3 {
		t.Fatal("thing")
	}
}
