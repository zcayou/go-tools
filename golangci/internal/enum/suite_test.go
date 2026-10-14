package enum_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEnum(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Enum Suite")
}
