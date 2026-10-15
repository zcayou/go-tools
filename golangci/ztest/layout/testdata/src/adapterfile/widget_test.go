package adapterfile_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Widget", func() {})

func TestAdapterFile(t *testing.T) {
	RunSpecs(t, "Adapter File Suite") // want `ginkgo-adapter-file: the Ginkgo adapter belongs in suite_test\.go`
}
