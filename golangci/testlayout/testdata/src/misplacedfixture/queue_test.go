package misplacedfixture_test

import "testing"

func TestQueue(t *testing.T) {
	if shared+registryDependency()+refusing() != 3 {
		t.Fatal("queue")
	}
}
