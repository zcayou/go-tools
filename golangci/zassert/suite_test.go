package zassert_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestZassert(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Zassert Suite")
}
