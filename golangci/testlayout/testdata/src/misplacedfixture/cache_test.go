package misplacedfixture_test

import "testing"

func TestCache(t *testing.T) {
	if newStore()+cacheSize() != 3 {
		t.Fatal("cache")
	}
}
