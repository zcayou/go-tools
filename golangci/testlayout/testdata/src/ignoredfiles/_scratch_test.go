// The go tool ignores a file whose name starts with an underscore, so the
// adapter here is not in the test binary either.
package ignoredfiles_test

import "testing"

func TestScratch(t *testing.T) {
	RunSpecs(t, "Ignored Files Scratch Suite")
}
