package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("Rule", func() {
	DescribeTable("names each rule by the token its messages open with",
		func(rule engine.Rule, expected string) {
			Expect(rule.String()).To(Equal(expected))
		},
		Entry(nil, engine.RuleStringEnum, "string-enum"),
		Entry(nil, engine.RuleUnnamed, "unnamed"),
		Entry(nil, engine.RuleUnreadableName, "unreadable-name"),
		Entry(nil, engine.RuleAmbiguousName, "ambiguous-name"),
		Entry(nil, engine.RuleZeroMember, "zero-member"),
		Entry(nil, engine.RuleRestated, "restated"),
		Entry(nil, engine.RuleTokenDecision, "token-decision"),
		Entry(nil, engine.RuleForgedConversion, "forged-conversion"),
		Entry(nil, engine.RuleNumericVerb, "numeric-verb"),
		Entry(nil, engine.RuleStringBypass, "string-bypass"),
		Entry(nil, engine.RuleLaunderedConversion, "laundered-conversion"),
		Entry(nil, engine.RuleOrderedComparison, "ordered-comparison"),
	)
})
