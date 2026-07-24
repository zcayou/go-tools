package adapterduplicate_test

import "testing"

func TestSuite(t *testing.T) {
	RunSpecs(t, "Suite") // want `ginkgo-adapter-duplicate: a second file calls RunSpecs; one test binary runs one suite, so gadget_test\.go already runs these specs`
}
