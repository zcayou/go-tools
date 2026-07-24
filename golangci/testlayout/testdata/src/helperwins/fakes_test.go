package helperwins_test // want `specs-in-helper-file: a file named for its supporting role carries specs; move them to a file named <source>_test\.go, suite_test\.go`

import "testing"

func TestFake(t *testing.T) {
	t.Log("fakes.go sits beside this file, and the helper pattern still wins")
}
