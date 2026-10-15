package ginkgoimports_test

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.BeforeSuite(func() {})

func TestGinkgoImports(t *testing.T) {
	ginkgo.RunSpecs(t, "Ginkgo Imports Suite")
}
