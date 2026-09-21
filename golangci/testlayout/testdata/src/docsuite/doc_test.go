package docsuite_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Widget", func() {})

func TestDocSuite(t *testing.T) {
	RunSpecs(t, "Doc Suite") // want `ginkgo-adapter-file: the Ginkgo adapter belongs in suite_test\.go`
}
