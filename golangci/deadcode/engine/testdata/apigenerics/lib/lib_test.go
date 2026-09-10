package lib

import "testing"

// The tests are the module's only instantiation sites, which is the shape
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

// fake implements the sealed Source from a test file.
type fake struct{}

func (fake) source() string { return fakeHelper() }

func TestProject(t *testing.T) {
	if Project[int](NewValue[int]("value")) != "value" {
		t.Fatal("wrong value")
	}
	if Project[int](fake{}) != "fake" {
		t.Fatal("wrong fake")
	}
}
