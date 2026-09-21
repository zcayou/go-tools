// Nothing here reaches Ginkgo: the harness's RunSpecs is a method, and the bare
// Describe is the package's own. A blank import of Ginkgo brings none of its
// names into the file.
package lookalikes_test

import (
	"testing"

	_ "github.com/onsi/ginkgo/v2"

	"lookalikes"
)

type harness struct{}

func (harness) RunSpecs(t *testing.T, description string) { t.Log(description) }

var _ = Describe("Widget", func() {})

func TestWidget(t *testing.T) {
	if lookalikes.Widget() != 1 {
		t.Fatal("widget")
	}
	harness{}.RunSpecs(t, "Lookalikes")
}
