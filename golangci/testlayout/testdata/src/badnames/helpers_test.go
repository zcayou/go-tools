package badnames_test // want `specs-in-helper-file: a file named for its supporting role carries specs; move them to a file named <source>_test\.go, suite_test\.go`

import "testing"

func TestHelpers(t *testing.T) { t.Log("a helper file has no business holding this") }
