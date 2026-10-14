package zenum_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestZenum(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Zenum Plugin Suite")
}
