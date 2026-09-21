// An adapter outside the files reserved for it, in a file of a kind that may
// not exist: only the kind is reported.
package disallowedextras // want `test-package: whitebox test files are not allowed; declare package disallowedextras_test`

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Widget", func() {})

func TestWidget(t *testing.T) {
	RunSpecs(t, "Disallowed Extras Suite")
}
