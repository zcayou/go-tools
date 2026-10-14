package wholemodule_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWholemodule(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wholemodule Suite")
}
