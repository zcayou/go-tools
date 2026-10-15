// The specs sit in the internal test package and the adapter in the external
// one, which is the split ginkgo-adapter-missing has to see across.
package mixed_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
)

func TestMixed(t *testing.T) {
	RunSpecs(t, "Mixed Suite")
}
