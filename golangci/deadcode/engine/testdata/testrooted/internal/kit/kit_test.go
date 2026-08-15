package kit

import "testing"

func TestHelper(t *testing.T) {
	if Helper() != 1 {
		t.Fatal("wrong")
	}
}
