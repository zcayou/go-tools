package fixtureunparsable

import "testing"

func TestOther(t *testing.T) {
	if other()+shared() != 2 {
		t.Fatal("other")
	}
}
