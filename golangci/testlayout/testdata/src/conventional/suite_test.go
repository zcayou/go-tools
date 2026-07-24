package conventional_test

import "testing"

var _ = BeforeSuite(func() {})

var _ = AfterSuite(func() {})

func TestConventional(t *testing.T) {
	RunSpecs(t, "Conventional Suite")
}
