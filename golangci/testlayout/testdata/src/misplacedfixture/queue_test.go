package misplacedfixture_test

import "testing"

func TestQueue(t *testing.T) {
	if shared != 1 {
		t.Fatal("queue")
	}
}
