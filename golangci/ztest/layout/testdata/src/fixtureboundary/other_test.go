package fixtureboundary_test

import (
	"testing"

	. "fixtureboundary"
)

func TestOther(t *testing.T) {
	if Dotted()+Other() != 4 {
		t.Fatal("other")
	}
}
