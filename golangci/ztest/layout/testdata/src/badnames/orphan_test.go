package badnames_test // want `test-file-name: not a name a blackbox test file may have; want <source>_test\.go, suite_test\.go, helpers_test\.go, fakes_test\.go`

import "testing"

func TestOrphan(t *testing.T) { t.Log("no orphan.go sits beside this file") }
