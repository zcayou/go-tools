package ztest_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/ztest"
	"github.com/zcayou/go-tools/golangci/ztest/layout"
)

var _ = Describe("ConfigError", func() {
	DescribeTable("renders the setting at fault and what is wrong with it",
		func(settings map[string]any, message string) {
			_, err := ztest.New(settings)

			Expect(err).To(MatchError(message))
		},
		Entry("undecodable settings",
			map[string]any{"nope": true},
			`ztest: decoding settings: json: unknown field "nope"`),
		Entry("a malformed pattern in a category",
			map[string]any{"blackbox": map[string]any{"helper-patterns": []string{"[helpers_test.go"}}},
			`ztest: blackbox.helper-patterns: pattern "[helpers_test.go": syntax error in pattern`),
		Entry("a malformed adapter pattern",
			map[string]any{"adapter-patterns": []string{"pkg/suite_test.go"}},
			`ztest: adapter-patterns: pattern "pkg/suite_test.go": matched against a file name, `+
				`which holds no path separator`),
		Entry("no kind allowed",
			map[string]any{
				"whitebox":   map[string]any{"allowed": false},
				"blackbox":   map[string]any{"allowed": false},
				"standalone": map[string]any{"allowed": false},
			},
			"ztest: no kind of test file is allowed, so none could exist"),
		Entry("no adapter name",
			map[string]any{"adapter-patterns": []string{}},
			"ztest: adapter-patterns is empty, so no file could hold the Ginkgo adapter"),
		Entry("an allowed kind with no name",
			map[string]any{"blackbox": map[string]any{"patterns": []string{}, "helper-patterns": []string{}}},
			"ztest: blackbox is allowed but no pattern permits any name"),
		Entry("a source pattern for files beside no source",
			map[string]any{"standalone": map[string]any{"patterns": []string{"<source>_test.go"}}},
			`ztest: standalone.patterns: pattern "<source>_test.go": <source> names a source file, `+
				`and no file of this kind is named for one`),
	)

	DescribeTable("renders what it can of a value no setting produced",
		func(err *ztest.ConfigError, message string) {
			Expect(err).To(MatchError(message))
		},
		Entry("undecodable, with nothing said",
			&ztest.ConfigError{Problem: ztest.ProblemUndecodable},
			"ztest: settings do not decode"),
		Entry("a malformed pattern, with nothing said",
			&ztest.ConfigError{
				Kind: layout.KindBlackbox, Setting: ztest.SettingPatterns,
				Problem: ztest.ProblemMalformedPattern,
			},
			"ztest: blackbox.patterns: a pattern no file name could match"),
		Entry("a problem outside the enum",
			&ztest.ConfigError{},
			"ztest: Setting(0): Problem(0)"),
	)

	It("renders an empty api", func() {
		_, err := ztest.New(map[string]any{"api": []string{}})

		Expect(err).To(MatchError(
			"ztest: api: an empty list names no package; leave the key out to judge every API package"))
	})

	It("renders a malformed rendering pattern with what the grammar said", func() {
		_, err := ztest.New(map[string]any{"rendering-patterns": []string{"pkg/<source>_test.go"}})

		Expect(err).To(MatchError(`ztest: rendering-patterns: pattern "pkg/<source>_test.go": ` +
			"matched against a file name, which holds no path separator"))
	})

	It("renders a rendering pattern holding no <source>", func() {
		_, err := ztest.New(map[string]any{"rendering-patterns": []string{"*_test.go"}})

		Expect(err).To(MatchError(`ztest: rendering-patterns: pattern "*_test.go" holds no <source>, ` +
			"so it names no source file a test file could pin the rendering of"))
	})

	It("renders an api pattern the module does not resolve with what the loader said", func() {
		dir := apiscopeDir()
		chdir(dir)
		analyzer, err := ztest.NewAnalyzer(ztest.Settings{API: []string{"./empty/..."}})
		Expect(err).NotTo(HaveOccurred())

		_, err = runPass(analyzer, loadPasses(dir)[0], dir)

		Expect(err).To(MatchError("ztest: api: pattern ./empty/... matched no packages"))
	})

	It("renders a missing module root with where the walk started", func() {
		dir := apiscopeDir()
		pkgs := loadPasses(dir)
		analyzer, err := ztest.NewAnalyzer(ztest.Settings{API: []string{"./narrow"}})
		Expect(err).NotTo(HaveOccurred())
		anchorless := anchorlessDir()
		chdir(anchorless)

		_, err = runPass(analyzer, pkgs[0], dir)

		Expect(err).To(MatchError(
			"ztest: api: resolving its patterns needs the module root: no go.mod at or above " + anchorless))
	})

	It("renders a rule selection as the selection says it", func() {
		_, err := ztest.New(map[string]any{"disable": []string{"nonesuch"}})

		Expect(err).To(MatchError(`ztest: disable: "nonesuch" is no rule of this linter`))
	})

	DescribeTable("renders a setting whose every rule the selection leaves off",
		func(err *ztest.ConfigError, message string) {
			Expect(err).To(MatchError(message))
		},
		Entry("a category block",
			&ztest.ConfigError{Kind: layout.KindStandalone, Problem: ztest.ProblemServesNoRule},
			"ztest: standalone configures only the layout rules, which the rule selection leaves off"),
		Entry("rendering-patterns",
			&ztest.ConfigError{Setting: ztest.SettingRenderingPatterns, Problem: ztest.ProblemServesNoRule},
			"ztest: rendering-patterns configures only text-assertion and token-assertion, "+
				"which the rule selection leaves off"),
	)

	DescribeTable("renders what it can of a spec setting no value produced",
		func(err *ztest.ConfigError, message string) {
			Expect(err).To(MatchError(message))
		},
		Entry("no module root, with nothing said",
			&ztest.ConfigError{Setting: ztest.SettingAPI, Problem: ztest.ProblemNoModuleRoot},
			"ztest: api: resolving its patterns needs the module root"),
		Entry("an unresolved api pattern, with nothing said",
			&ztest.ConfigError{Setting: ztest.SettingAPI, Problem: ztest.ProblemUnresolvedAPIPattern, Pattern: "./x"},
			`ztest: api: pattern "./x" matches no API package`),
		Entry("a rule selection, with nothing said",
			&ztest.ConfigError{Problem: ztest.ProblemRuleSelection},
			"ztest: the rule selection means nothing"),
	)

	It("names each setting and problem by its settings token", func() {
		Expect([]string{
			ztest.SettingPatterns.String(), ztest.SettingHelperPatterns.String(), ztest.SettingAdapterPatterns.String(),
			ztest.SettingAPI.String(), ztest.SettingRenderingPatterns.String(), ztest.SettingBuildTags.String(),
		}).To(Equal([]string{
			"patterns", "helper-patterns", "adapter-patterns", "api", "rendering-patterns", "build-tags",
		}))
		Expect([]string{
			ztest.ProblemUndecodable.String(), ztest.ProblemMalformedPattern.String(),
			ztest.ProblemNoKindAllowed.String(), ztest.ProblemNoAdapterName.String(),
			ztest.ProblemNoName.String(), ztest.ProblemPatternNamesSource.String(),
			ztest.ProblemPatternNamesNoSource.String(), ztest.ProblemEmptyAPI.String(),
			ztest.ProblemNoModuleRoot.String(), ztest.ProblemUnresolvedAPIPattern.String(),
			ztest.ProblemRuleSelection.String(), ztest.ProblemServesNoRule.String(),
		}).To(Equal([]string{
			"undecodable", "malformed-pattern", "no-kind-allowed", "no-adapter-name", "no-name",
			"pattern-names-source", "pattern-names-no-source", "empty-api", "no-module-root",
			"unresolved-api-pattern", "rule-selection", "serves-no-rule",
		}))
	})
})
