package filepattern_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFilepattern(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filepattern Suite")
}
