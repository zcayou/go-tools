package zenum_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum"
	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("ConfigError", func() {
	It("renders a rule selection as the selection says it", func() {
		_, err := zenum.New(map[string]any{"default": "most"})

		Expect(err).To(MatchError(`zenum: default: "most" is none of standard, all and none`))
	})

	It("renders a decoding failure as decoding said it, once", func() {
		// register.DecodeSettings already says what it was doing, and golangci-lint
		// prefixes the plugin name on top of that.
		_, err := zenum.New(map[string]any{"nope": true})

		Expect(err).To(MatchError(`decoding settings: json: unknown field "nope"`))
		Expect(errors.Unwrap(err)).To(MatchError(HavePrefix("decoding settings: ")))
	})

	DescribeTable("renders the entry and what is wrong with it",
		func(err *zenum.ConfigError, expected string) {
			Expect(err.Error()).To(Equal(expected))
		},
		Entry("an empty reason",
			&zenum.ConfigError{
				Problem: zenum.ProblemUnexplainedException, Setting: engine.SettingZeroDefaults, Index: 2,
			},
			"zenum: zero-defaults[2]: reason is empty: say why the entry holds"),
		Entry("a type not named as import/path.Name",
			&zenum.ConfigError{
				Problem: zenum.ProblemUnqualifiedType, Setting: engine.SettingOpenEnums, Type: "ID",
			},
			`zenum: open-enums[0]: "ID" does not name a type as import/path.Name`),
		Entry("a rule selection with no cause",
			&zenum.ConfigError{Problem: zenum.ProblemRuleSelection},
			"zenum: the rule selection means nothing"),
		Entry("a setting for rules that are off",
			&zenum.ConfigError{Problem: zenum.ProblemServesNoRule, Setting: engine.SettingZeroDefaults},
			"zenum: zero-defaults configures only zero-member, which the rule selection leaves off"),
		Entry("a setting for two rules that are off",
			&zenum.ConfigError{Problem: zenum.ProblemServesNoRule, Setting: engine.SettingOrderedEnums},
			"zenum: ordered-enums configures only ordered-comparison and member-bound, "+
				"which the rule selection leaves off"),
		Entry("a setting for every use-site rule, all off",
			&zenum.ConfigError{Problem: zenum.ProblemServesNoRule, Setting: engine.SettingTests},
			"zenum: tests configures only token-decision, forged-conversion, laundered-conversion, numeric-verb, "+
				"string-bypass, ordered-comparison and member-bound, which the rule selection leaves off"),
		Entry("a decoding failure with no cause",
			&zenum.ConfigError{Problem: zenum.ProblemUndecodableSettings},
			"zenum: settings do not decode"),
		Entry("no problem or setting at all",
			&zenum.ConfigError{},
			"zenum: Setting(0)[0]: Problem(0)"),
	)

	DescribeTable("names each problem",
		func(problem zenum.Problem, expected string) {
			Expect(problem.String()).To(Equal(expected))
		},
		Entry(nil, zenum.ProblemUndecodableSettings, "undecodable-settings"),
		Entry(nil, zenum.ProblemUnexplainedException, "unexplained-exception"),
		Entry(nil, zenum.ProblemUnqualifiedType, "unqualified-type"),
		Entry(nil, zenum.ProblemRuleSelection, "rule-selection"),
		Entry(nil, zenum.ProblemServesNoRule, "serves-no-rule"),
	)
})
