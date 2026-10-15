package ruleset_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRuleset(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ruleset Suite")
}
