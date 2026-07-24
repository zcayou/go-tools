package engine_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDeadcode(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Deadcode Suite")
}
