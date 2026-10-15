// The category names its own patterns, so the default helpers_test.go is not
// among them.
package whiteboxcustom // want `test-file-name: not a name a whitebox test file may have; want <source>_internal_test\.go, internal_helpers_test\.go`

func tripled(v int) int { return v * 3 }
