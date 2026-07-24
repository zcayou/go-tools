package blackboxdenied_test // want `test-package: blackbox test files are not allowed; declare package blackboxdenied`

import (
	"testing"

	"blackboxdenied"
)

func TestThing(t *testing.T) {
	if blackboxdenied.Thing() != 1 {
		t.Fatal("thing")
	}
}
