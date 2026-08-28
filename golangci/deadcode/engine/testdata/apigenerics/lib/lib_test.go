package lib

import "testing"

// TestStore is the module's only instantiation site, which is the shape
// a library has: consumers instantiate its generics, and it does not.
func TestStore(t *testing.T) {
	store := Load([]int{1})
	store.Put(2)
	if Merge([]int{2}, []int{3}) == nil {
		t.Fatal("nil store")
	}
	if auditHelper() != "audited" {
		t.Fatal("wrong audit")
	}
}
