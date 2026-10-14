package engine_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("Analyze", func() {
	It("fails on every exception that suppresses nothing at once", func() {
		err := analyzeError("restated", engine.Config{
			OpenEnums: []string{"example.com/restated/a.Missing"},
			RelatedEnums: []engine.Pair{
				{First: "example.com/restated/a.Solo", Second: "example.com/restated/b.Lone"},
			},
		})

		joined, ok := errors.AsType[interface {
			error
			Unwrap() []error
		}](err)
		Expect(ok).To(BeTrue())
		Expect(joined.Unwrap()).To(ConsistOf(
			&engine.ExceptionError{
				Setting: engine.SettingOpenEnums, Fault: engine.FaultNotEnum,
				Type: "example.com/restated/a.Missing",
			},
			&engine.ExceptionError{
				Setting: engine.SettingRelatedEnums, Fault: engine.FaultNotRestated,
				Type: "example.com/restated/a.Solo", Related: "example.com/restated/b.Lone",
			},
		))
	})

	It("opens a finding's message with its rule", func() {
		finding := engine.Finding{Rule: engine.RuleRestated, Detail: "b.Paint restates the tokens of a.Color"}

		Expect(finding.Message()).To(Equal("restated: b.Paint restates the tokens of a.Color"))
	})
})
