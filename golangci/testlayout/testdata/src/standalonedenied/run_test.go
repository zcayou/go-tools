package standalonedenied // want `test-package: standalone test files are not allowed; a test file sits beside the source it exercises`

import "testing"

func TestRun(t *testing.T) { t.Log("nothing beside this file is what it exercises") }
