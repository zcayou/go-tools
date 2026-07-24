package zlines_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestZlines(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Zlines Suite")
}
