package customnames_test

import (
	"testing"

	"customnames"
)

func TestThing(t *testing.T) {
	if customnames.Thing() != 1 {
		t.Fatal("thing")
	}
}
