package zlines_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zlines"
)

var _ = Describe("ConfigError", func() {
	DescribeTable("renders the setting at fault and what is wrong with it",
		func(settings map[string]any, message string) {
			_, err := zlines.New(settings)

			Expect(err).To(MatchError(message))
		},
		Entry("undecodable settings", map[string]any{"max-line-length": 160},
			`zlines: decoding settings: json: unknown field "max-line-length"`),
		Entry("a length that is not positive", map[string]any{"line-length": 0},
			"zlines: line-length must be positive, got 0"),
		Entry("a negative overrun", map[string]any{"comment-overrun": -1},
			"zlines: comment-overrun must not be negative, got -1"),
		Entry("a minimum past the length", map[string]any{"comment-length": 80, "comment-min-length": 90},
			"zlines: comment-min-length 90 exceeds comment-length 80"),
		Entry("an empty exempt prefix", map[string]any{"comment-exempt": []string{""}},
			"zlines: comment-exempt must not hold an empty prefix"),
	)

	DescribeTable("renders what it can of a value no setting produced",
		func(err *zlines.ConfigError, message string) {
			Expect(err).To(MatchError(message))
		},
		Entry("undecodable, with nothing said",
			&zlines.ConfigError{Problem: zlines.ProblemUndecodable}, "zlines: settings do not decode"),
		Entry("a problem outside the enum", &zlines.ConfigError{}, "zlines: Setting(0): Problem(0)"),
	)

	It("names each setting and problem by its token", func() {
		Expect([]string{
			zlines.SettingLineLength.String(), zlines.SettingCommentLength.String(),
			zlines.SettingCommentMinLength.String(), zlines.SettingCommentOverrun.String(),
			zlines.SettingCommentExempt.String(),
		}).To(Equal([]string{"line-length", "comment-length", "comment-min-length", "comment-overrun", "comment-exempt"}))
		Expect([]string{
			zlines.ProblemUndecodable.String(), zlines.ProblemNotPositive.String(), zlines.ProblemNegative.String(),
			zlines.ProblemMinimumPastLength.String(), zlines.ProblemEmptyPrefix.String(),
		}).To(Equal([]string{"undecodable", "not-positive", "negative", "minimum-past-length", "empty-prefix"}))
	})
})
