package standalonenames // want `test-file-name: not a name a standalone test file may have; want \*_integration_test\.go, helpers_test\.go, fakes_test\.go`

import "testing"

func TestOrphan(t *testing.T) { t.Log("named for nothing the patterns allow") }
