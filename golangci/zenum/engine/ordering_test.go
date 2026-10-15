package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("ordered comparisons", func() {
	It("reports an ordering of an enum outside its package", func() {
		Expect(analyzeFixture("ordered", engine.Config{})).To(Equal([]string{
			// A loop bound, each ordering operator, and min and max. Equality,
			// a method deciding membership, a sort, an ordering of a type parameter,
			// and the declaring package's own orderings are clean.
			"ordered.go:10:28: ordered-comparison: example.com/ordered/kinds.State",
			"ordered.go:15:5: ordered-comparison: example.com/ordered/kinds.State",
			"ordered.go:16:17: ordered-comparison: example.com/ordered/kinds.State",
			"ordered.go:17:5: ordered-comparison: example.com/ordered/kinds.State",
			"ordered.go:18:3: ordered-comparison: example.com/ordered/kinds.State",
			"ordered.go:19:3: ordered-comparison: example.com/ordered/kinds.State",
			"ordered.go:22:5: ordered-comparison: example.com/ordered/kinds.Level",
		}))
	})

	It("says where membership belongs", func() {
		Expect(messages("ordered", engine.Config{})).To(HaveKeyWithValue("ordered.go:15:5",
			"ordered-comparison: orders kinds.State, whose ordinals are an encoding rather than an order; "+
				"decide membership in example.com/ordered/kinds, or list it in ordered-enums"))
	})

	It("leaves an enum ordered-enums lists alone", func() {
		Expect(analyzeFixture("ordered", engine.Config{OrderedEnums: []string{"example.com/ordered/kinds.Level"}})).
			NotTo(ContainElement(HaveSuffix("kinds.Level")))
	})

	DescribeTable("fails on an ordered-enums entry that suppresses nothing",
		func(name string, expected *engine.ExceptionError) {
			Expect(analyzeError("ordered", engine.Config{OrderedEnums: []string{name}})).To(MatchError(expected))
		},
		Entry("a type that is no enum", "example.com/ordered/kinds.Missing", &engine.ExceptionError{
			Setting: engine.SettingOrderedEnums, Fault: engine.FaultNotEnum, Type: "example.com/ordered/kinds.Missing",
		}),
		Entry("an enum nothing outside its package orders", "example.com/ordered/kinds.Spare", &engine.ExceptionError{
			Setting: engine.SettingOrderedEnums, Fault: engine.FaultNotOrdered, Type: "example.com/ordered/kinds.Spare",
		}),
	)
})
