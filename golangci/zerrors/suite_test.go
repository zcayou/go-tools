package zerrors_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestZerrors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Zerrors Plugin Suite")
}
