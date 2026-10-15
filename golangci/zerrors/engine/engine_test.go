package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("Analyze", func() {
	It("runs no rule the configuration disables", func() {
		findings, err := run("literal", engine.Config{Disabled: []engine.Rule{engine.RuleLiteralField}})
		Expect(err).NotTo(HaveOccurred())

		Expect(findings).NotTo(ContainElement(HaveField("Rule", engine.RuleLiteralField)))
		Expect(analyze("literal", engine.Config{}, engine.RuleLiteralField)).NotTo(BeEmpty())
	})
})
