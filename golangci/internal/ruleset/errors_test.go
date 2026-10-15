package ruleset_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/ruleset"
)

var _ = Describe("Error", func() {
	DescribeTable("says what is wrong with the selection",
		func(err *ruleset.Error, text string) {
			Expect(err.Error()).To(Equal(text))
		},
		Entry("an unknown default",
			&ruleset.Error{Key: ruleset.KeyDefault, Problem: ruleset.ProblemUnknownDefault, Value: "most"},
			`default: "most" is none of standard, all and none`),
		Entry("an unknown rule",
			&ruleset.Error{Key: ruleset.KeyEnable, Problem: ruleset.ProblemUnknownRule, Value: "third"},
			`enable: "third" is no rule of this linter`),
		Entry("a rule listed twice",
			&ruleset.Error{Key: ruleset.KeyDisable, Problem: ruleset.ProblemRepeatedRule, Value: "first"},
			`disable: "first" is listed twice`),
		Entry("a rule in both lists",
			&ruleset.Error{Key: ruleset.KeyEnable, Problem: ruleset.ProblemEnabledAndDisabled, Value: "first"},
			`enable: "first" is also listed under disable`),
		Entry("a rule already enabled",
			&ruleset.Error{Key: ruleset.KeyEnable, Problem: ruleset.ProblemAlreadyEnabled, Value: "first"},
			`enable: "first" is already enabled by default`),
		Entry("a rule not enabled",
			&ruleset.Error{Key: ruleset.KeyDisable, Problem: ruleset.ProblemNotEnabled, Value: "first"},
			`disable: "first" is not enabled by default, so there is nothing to disable`),
		Entry("an empty list",
			&ruleset.Error{Key: ruleset.KeyEnable, Problem: ruleset.ProblemEmptyList},
			"enable: an empty list names no rule; leave the key out"),
		Entry("no rule enabled",
			&ruleset.Error{Key: ruleset.KeyDefault, Problem: ruleset.ProblemNoRuleEnabled},
			"the selection enables no rule; disable the linter instead"),
		Entry("a problem out of range", &ruleset.Error{Key: ruleset.KeyDefault}, "default: Problem(0)"),
	)

	DescribeTable("names each key and problem",
		func(token string, rendered string) {
			Expect(rendered).To(Equal(token))
		},
		Entry(nil, "default", ruleset.KeyDefault.String()),
		Entry(nil, "enable", ruleset.KeyEnable.String()),
		Entry(nil, "disable", ruleset.KeyDisable.String()),
		Entry(nil, "Key(0)", ruleset.Key(0).String()),
		Entry(nil, "unknown-default", ruleset.ProblemUnknownDefault.String()),
		Entry(nil, "unknown-rule", ruleset.ProblemUnknownRule.String()),
		Entry(nil, "repeated-rule", ruleset.ProblemRepeatedRule.String()),
		Entry(nil, "enabled-and-disabled", ruleset.ProblemEnabledAndDisabled.String()),
		Entry(nil, "already-enabled", ruleset.ProblemAlreadyEnabled.String()),
		Entry(nil, "not-enabled", ruleset.ProblemNotEnabled.String()),
		Entry(nil, "empty-list", ruleset.ProblemEmptyList.String()),
		Entry(nil, "no-rule-enabled", ruleset.ProblemNoRuleEnabled.String()),
	)
})
