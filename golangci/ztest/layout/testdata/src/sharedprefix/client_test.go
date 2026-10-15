// client_options.go only shares a prefix with this file. It is not client.go
// under a GOOS or GOARCH name, so there is no client for client_test.go to be
// named after.
package sharedprefix_test // want `test-file-name: not a name a blackbox test file may have; want <source>_test\.go, suite_test\.go, helpers_test\.go, fakes_test\.go`

import "testing"

func TestClient(t *testing.T) { t.Log("no client.go sits beside this file") }
