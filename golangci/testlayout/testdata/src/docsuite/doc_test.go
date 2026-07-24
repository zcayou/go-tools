package docsuite_test

import "testing"

func Describe(text string, body func()) bool { body(); return true }

func RunSpecs(t *testing.T, description string) { t.Log(description) }

var _ = Describe("Widget", func() {})

func TestDocSuite(t *testing.T) {
	RunSpecs(t, "Doc Suite") // want `ginkgo-adapter-file: the Ginkgo adapter belongs in suite_test\.go`
}
