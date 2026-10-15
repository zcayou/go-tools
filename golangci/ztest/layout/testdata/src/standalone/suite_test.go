package standalone

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
)

var _ = BeforeSuite(func() {})

var _ = AfterSuite(func() {})

func TestStandalone(t *testing.T) {
	RunSpecs(t, "Standalone Suite")
}
