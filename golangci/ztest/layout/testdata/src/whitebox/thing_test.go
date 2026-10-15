package whitebox // want `test-package: whitebox test files are not allowed; declare package whitebox_test`

import "testing"

func TestThing(t *testing.T) {
	if thing() != 1 {
		t.Fatal("thing")
	}
}
