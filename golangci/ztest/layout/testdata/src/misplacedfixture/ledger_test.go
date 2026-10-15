package misplacedfixture_test

import "testing"

func TestLedger(t *testing.T) {
	if outer() != 1 {
		t.Fatal("ledger")
	}
}

// idle is never used, so what its method reaches is not consumed through it.
type idle struct{}

func (idle) at() int { return stamp() }
