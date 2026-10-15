package misplacedfixture_test

import "testing"

func TestCache(t *testing.T) {
	if newStore()+cacheSize()+offerScope()+(writer{}).write() != 5 {
		t.Fatal("cache")
	}
}

func (writer) write() int { return flushed() }
