package adapterfile_test

import "testing"

func Describe(text string, body func()) bool { body(); return true }

func RunSpecs(t *testing.T, description string) { t.Log(description) }

var _ = Describe("Widget", func() {})

func TestAdapterFile(t *testing.T) {
	RunSpecs(t, "Adapter File Suite") // want `ginkgo-adapter-file: the Ginkgo adapter belongs in suite_test\.go`
}
