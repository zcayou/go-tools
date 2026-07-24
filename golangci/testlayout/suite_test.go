package testlayout_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTestlayout(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testlayout Suite")
}
