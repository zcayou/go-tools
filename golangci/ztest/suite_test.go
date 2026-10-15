package ztest_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestZtest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ztest Suite")
}
