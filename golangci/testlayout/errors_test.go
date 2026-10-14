package testlayout_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/testlayout"
)

var _ = Describe("ConfigError", func() {
	DescribeTable("renders the setting at fault and what is wrong with it",
		func(settings map[string]any, message string) {
			_, err := testlayout.New(settings)

			Expect(err).To(MatchError(message))
		},
		Entry("undecodable settings",
			map[string]any{"nope": true},
			`testlayout: decoding settings: json: unknown field "nope"`),
		Entry("a malformed pattern in a category",
			map[string]any{"blackbox": map[string]any{"helper-patterns": []string{"[helpers_test.go"}}},
			`testlayout: blackbox.helper-patterns: pattern "[helpers_test.go": syntax error in pattern`),
		Entry("a malformed adapter pattern",
			map[string]any{"adapter-patterns": []string{"pkg/suite_test.go"}},
			`testlayout: adapter-patterns: pattern "pkg/suite_test.go": matched against a file name, `+
				`which holds no path separator`),
		Entry("no kind allowed",
			map[string]any{
				"whitebox":   map[string]any{"allowed": false},
				"blackbox":   map[string]any{"allowed": false},
				"standalone": map[string]any{"allowed": false},
			},
			"testlayout: no kind of test file is allowed, so none could exist"),
		Entry("no adapter name",
			map[string]any{"adapter-patterns": []string{}},
			"testlayout: adapter-patterns is empty, so no file could hold the Ginkgo adapter"),
		Entry("an allowed kind with no name",
			map[string]any{"blackbox": map[string]any{"patterns": []string{}, "helper-patterns": []string{}}},
			"testlayout: blackbox is allowed but no pattern permits any name"),
		Entry("a source pattern for files beside no source",
			map[string]any{"standalone": map[string]any{"patterns": []string{"<source>_test.go"}}},
			`testlayout: standalone.patterns: pattern "<source>_test.go": <source> names a source file, `+
				`and no file of this kind is named for one`),
	)

	DescribeTable("renders what it can of a value no setting produced",
		func(err *testlayout.ConfigError, message string) {
			Expect(err).To(MatchError(message))
		},
		Entry("undecodable, with nothing said",
			&testlayout.ConfigError{Problem: testlayout.ProblemUndecodable},
			"testlayout: settings do not decode"),
		Entry("a malformed pattern, with nothing said",
			&testlayout.ConfigError{
				Kind: testlayout.KindBlackbox, Setting: testlayout.SettingPatterns,
				Problem: testlayout.ProblemMalformedPattern,
			},
			"testlayout: blackbox.patterns: a pattern no file name could match"),
		Entry("a problem outside the enum",
			&testlayout.ConfigError{},
			"testlayout: Setting(0): Problem(0)"),
	)

	It("names each kind, setting and problem by its settings token", func() {
		Expect([]string{
			testlayout.KindWhitebox.String(), testlayout.KindBlackbox.String(), testlayout.KindStandalone.String(),
		}).To(Equal([]string{"whitebox", "blackbox", "standalone"}))
		Expect([]string{
			testlayout.SettingPatterns.String(), testlayout.SettingHelperPatterns.String(),
			testlayout.SettingAdapterPatterns.String(),
		}).To(Equal([]string{"patterns", "helper-patterns", "adapter-patterns"}))
		Expect([]string{
			testlayout.ProblemUndecodable.String(), testlayout.ProblemMalformedPattern.String(),
			testlayout.ProblemNoKindAllowed.String(), testlayout.ProblemNoAdapterName.String(),
			testlayout.ProblemNoName.String(), testlayout.ProblemSourcelessPattern.String(),
		}).To(Equal([]string{
			"undecodable", "malformed-pattern", "no-kind-allowed", "no-adapter-name", "no-name",
			"sourceless-pattern",
		}))
	})
})
