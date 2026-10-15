package ruleset_test

import (
	"slices"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/ruleset"
)

type rule int

const (
	ruleFirst rule = iota + 1
	ruleSecond
	ruleOptIn
)

func (r rule) String() string {
	switch r {
	case ruleFirst:
		return "first"
	case ruleSecond:
		return "second"
	case ruleOptIn:
		return "opt-in"
	default:
		return "rule(" + strconv.Itoa(int(r)) + ")"
	}
}

var catalog = ruleset.Catalog[rule]{Rules: []rule{ruleFirst, ruleSecond, ruleOptIn}, OptIn: []rule{ruleOptIn}}

var _ = Describe("Catalog", func() {
	DescribeTable("resolves a selection to the rules it runs",
		func(selection ruleset.Selection, enabled []rule) {
			got, err := catalog.Resolve(selection)

			Expect(err).NotTo(HaveOccurred())
			for _, r := range catalog.Rules {
				Expect(got.Has(r)).To(Equal(slices.Contains(enabled, r)), r.String())
			}
		},
		Entry("an absent selection, which is standard", ruleset.Selection{}, []rule{ruleFirst, ruleSecond}),
		Entry("standard written out", ruleset.Selection{Default: new(ruleset.Standard)}, []rule{ruleFirst, ruleSecond}),
		Entry("all", ruleset.Selection{Default: new(ruleset.All)}, []rule{ruleFirst, ruleSecond, ruleOptIn}),
		Entry("none, with a rule enabled",
			ruleset.Selection{Default: new(ruleset.None), Enable: []string{"second"}}, []rule{ruleSecond}),
		Entry("an opt-in rule enabled", ruleset.Selection{Enable: []string{"opt-in"}},
			[]rule{ruleFirst, ruleSecond, ruleOptIn}),
		Entry("a standard rule disabled", ruleset.Selection{Disable: []string{"first"}}, []rule{ruleSecond}),
	)

	DescribeTable("rejects a selection that means nothing",
		func(selection ruleset.Selection, expected *ruleset.Error) {
			_, err := catalog.Resolve(selection)

			Expect(err).To(MatchError(expected))
		},
		Entry("an unknown default",
			ruleset.Selection{Default: new("most")},
			&ruleset.Error{Key: ruleset.KeyDefault, Problem: ruleset.ProblemUnknownDefault, Value: "most"}),
		Entry("an unknown rule",
			ruleset.Selection{Disable: []string{"third"}},
			&ruleset.Error{Key: ruleset.KeyDisable, Problem: ruleset.ProblemUnknownRule, Value: "third"}),
		Entry("a rule listed twice",
			ruleset.Selection{Default: new(ruleset.None), Enable: []string{"first", "first"}},
			&ruleset.Error{Key: ruleset.KeyEnable, Problem: ruleset.ProblemRepeatedRule, Value: "first"}),
		Entry("a rule both enabled and disabled",
			ruleset.Selection{Enable: []string{"opt-in"}, Disable: []string{"opt-in"}},
			&ruleset.Error{Key: ruleset.KeyEnable, Problem: ruleset.ProblemEnabledAndDisabled, Value: "opt-in"}),
		Entry("an enable of a rule the default enables",
			ruleset.Selection{Enable: []string{"first"}},
			&ruleset.Error{Key: ruleset.KeyEnable, Problem: ruleset.ProblemAlreadyEnabled, Value: "first"}),
		Entry("a disable of a rule the default leaves out",
			ruleset.Selection{Disable: []string{"opt-in"}},
			&ruleset.Error{Key: ruleset.KeyDisable, Problem: ruleset.ProblemNotEnabled, Value: "opt-in"}),
		Entry("an empty enable",
			ruleset.Selection{Enable: []string{}},
			&ruleset.Error{Key: ruleset.KeyEnable, Problem: ruleset.ProblemEmptyList}),
		Entry("an empty disable",
			ruleset.Selection{Disable: []string{}},
			&ruleset.Error{Key: ruleset.KeyDisable, Problem: ruleset.ProblemEmptyList}),
		Entry("none, alone",
			ruleset.Selection{Default: new(ruleset.None)},
			&ruleset.Error{Key: ruleset.KeyDefault, Problem: ruleset.ProblemNoRuleEnabled}),
		Entry("every standard rule disabled",
			ruleset.Selection{Disable: []string{"first", "second"}},
			&ruleset.Error{Key: ruleset.KeyDefault, Problem: ruleset.ProblemNoRuleEnabled}),
	)

	It("lets a linter with findings beside its rules enable none", func() {
		beside := ruleset.Catalog[rule]{Rules: []rule{ruleOptIn}, OptIn: []rule{ruleOptIn}, Beside: true}

		got, err := beside.Resolve(ruleset.Selection{})

		Expect(err).NotTo(HaveOccurred())
		Expect(got.Has(ruleOptIn)).To(BeFalse())
	})
})

var _ = Describe("Enabled", func() {
	It("answers for any of several rules, and lists the rest", func() {
		enabled, err := catalog.Resolve(ruleset.Selection{Disable: []string{"first"}})
		Expect(err).NotTo(HaveOccurred())

		Expect(enabled.Any(ruleFirst, ruleSecond)).To(BeTrue())
		Expect(enabled.Any(ruleFirst, ruleOptIn)).To(BeFalse())
		Expect(enabled.Disabled(catalog)).To(Equal([]rule{ruleFirst, ruleOptIn}))
	})

	It("enables nothing as its zero value", func() {
		Expect(ruleset.Enabled[rule]{}.Has(ruleFirst)).To(BeFalse())
	})
})
