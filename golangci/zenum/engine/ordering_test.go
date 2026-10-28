package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("ordered comparisons", func() {
	It("reports an ordering of an enum outside its package", func() {
		Expect(analyzeFixture("ordered", engine.Config{})).To(Equal([]string{
			// The declaring package bounds State by its highest member, which
			// is member-bound's; its other orderings are clean.
			"kinds/kinds.go:26:43: member-bound: example.com/ordered/kinds.State",
			// A loop bound, each ordering operator, and min and max. Equality,
			// a method deciding membership, a sort and an ordering of a type parameter
			// are clean.
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

var _ = Describe("member bounds", func() {
	It("reports an ordering against the highest member inside the declaring package", func() {
		Expect(analyzeFixture("memberbound", engine.Config{})).To(Equal([]string{
			// Valid's upper bound, an out-of-range check, and max. A lower bound
			// by the first member, a bound by a count sentinel and a switch are clean.
			"kinds/kinds.go:25:53: member-bound: example.com/memberbound/kinds.Kind",
			"kinds/kinds.go:28:54: member-bound: example.com/memberbound/kinds.Kind",
			"kinds/kinds.go:31:33: member-bound: example.com/memberbound/kinds.Kind",
			"kinds/kinds.go:77:41: member-bound: example.com/memberbound/kinds.Level",
			// Outside the declaring package, every ordering is ordered-comparison's.
			"user/user.go:5:44: ordered-comparison: example.com/memberbound/kinds.Kind",
		}))
	})

	It("says what the bound leaves out", func() {
		Expect(messages("memberbound", engine.Config{})).To(HaveKeyWithValue("kinds/kinds.go:25:53",
			"member-bound: bounds Kind by KindC, its highest member, so a member added after KindC falls outside; "+
				"bound it by a count sentinel, or switch over the members"))
	})

	It("leaves an enum ordered-enums lists alone, and counts the bound it excuses", func() {
		Expect(analyzeFixture("memberbound", engine.Config{OrderedEnums: []string{"example.com/memberbound/kinds.Level"}})).
			NotTo(ContainElement(HaveSuffix("kinds.Level")))
	})

	It("counts only the bounds a running rule judges as excused", func() {
		Expect(analyzeError("memberbound", engine.Config{
			OrderedEnums: []string{"example.com/memberbound/kinds.Level"},
			Disabled:     []engine.Rule{engine.RuleMemberBound},
		})).To(MatchError(&engine.ExceptionError{
			Setting: engine.SettingOrderedEnums, Fault: engine.FaultNotOrdered, Type: "example.com/memberbound/kinds.Level",
		}))
	})
})

var _ = Describe("ordered instantiations", func() {
	instantiations := engine.Config{OrderedInstantiations: true}

	It("judges no instantiation unless told to", func() {
		Expect(analyzeFixture("instantiations", engine.Config{})).To(BeNil())
	})

	It("reports an enum passed outside its package to a type parameter whose constraint orders it", func() {
		Expect(analyzeFixture("instantiations", instantiations)).To(Equal([]string{
			// A generic type over cmp.Ordered, as a result and as a literal.
			"use/use.go:23:82: ordered-comparison: example.com/instantiations/kinds.Kind",
			// slices.Sort, slices.Sorted over maps.Keys, cmp.Compare in a comparator,
			// a module Max, and a constraint listing integer types. maps.Keys,
			// slices.SortFunc and slices.Contains constrain nothing to be ordered,
			// and a comparator comparing tokens orders no ordinal.
			"use/use.go:24:9: ordered-comparison: example.com/instantiations/kinds.Kind",
			"use/use.go:25:13: ordered-comparison: example.com/instantiations/kinds.Kind",
			"use/use.go:26:61: ordered-comparison: example.com/instantiations/kinds.Kind",
			"use/use.go:27:6: ordered-comparison: example.com/instantiations/kinds.Kind",
			"use/use.go:28:6: ordered-comparison: example.com/instantiations/kinds.Kind",
			"use/use.go:29:9: ordered-comparison: example.com/instantiations/kinds.Level",
			"use/use.go:32:9: ordered-comparison: example.com/instantiations/kinds.Kind",
		}))
	})

	It("names the generic ordering it", func() {
		Expect(messages("instantiations", instantiations)).To(HaveKeyWithValue("use/use.go:24:9",
			"ordered-comparison: orders kinds.Kind through slices.Sort, whose ordinals are an encoding rather than "+
				"an order; order by its tokens, or list it in ordered-enums"))
	})

	It("leaves an enum ordered-enums lists alone, and counts the instantiation it excuses", func() {
		ordered := []string{"example.com/instantiations/kinds.Level"}

		Expect(analyzeFixture("instantiations", engine.Config{OrderedInstantiations: true, OrderedEnums: ordered})).
			NotTo(ContainElement(HaveSuffix("kinds.Level")))
		Expect(analyzeError("instantiations", engine.Config{OrderedEnums: ordered})).To(MatchError(
			&engine.ExceptionError{
				Setting: engine.SettingOrderedEnums, Fault: engine.FaultNotOrdered,
				Type: "example.com/instantiations/kinds.Level",
			}))
	})

	It("judges the sorts of the ordered fixture too, once told to", func() {
		Expect(analyzeFixture("ordered", instantiations)).To(ContainElements(
			"ordered.go:13:9: ordered-comparison: example.com/ordered/kinds.State",
			"ordered.go:23:3: ordered-comparison: example.com/ordered/kinds.State",
		))
	})
})
