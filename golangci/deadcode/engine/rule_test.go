package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

var _ = Describe("Rule", func() {
	It("names each rule by its token", func() {
		Expect(engine.RuleInertField.String()).To(Equal("inert-field"))
		Expect(engine.RuleWriteOnlyField.String()).To(Equal("write-only-field"))
		Expect(engine.Rule(0).String()).To(Equal("Rule(0)"))
	})
})
