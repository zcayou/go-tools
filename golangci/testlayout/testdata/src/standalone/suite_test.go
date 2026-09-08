package standalone

import "testing"

var _ = BeforeSuite(func() {})

var _ = AfterSuite(func() {})

func TestStandalone(t *testing.T) {
	RunSpecs(t, "Standalone Suite")
}
