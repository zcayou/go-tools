package ztest_test

import (
	"errors"
	"path"

	"github.com/golangci/plugin-module-register/register"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/ruleset"
	"github.com/zcayou/go-tools/golangci/ztest"
	"github.com/zcayou/go-tools/golangci/ztest/internal/filepattern"
	"github.com/zcayou/go-tools/golangci/ztest/layout"
	"github.com/zcayou/go-tools/golangci/ztest/spec"
)

var _ = Describe("Plugin", func() {
	It("builds from an absent settings block", func() {
		plugin, err := ztest.New(nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("builds from an empty settings block", func() {
		plugin, err := ztest.New(map[string]any{})

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("rejects an unknown settings key rather than ignoring it", func() {
		_, err := ztest.New(map[string]any{"dedicated-test-dirs": []string{"integration"}})

		Expect(err).To(beConfigError(0, 0, ztest.ProblemUndecodable))
		Expect(errors.Unwrap(err)).To(MatchError(ContainSubstring(`unknown field "dedicated-test-dirs"`)))
	})

	It("rejects a malformed pattern rather than leaving it matching nothing", func() {
		_, err := ztest.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{"[_test.go"}},
		})

		Expect(err).To(beConfigError(layout.KindBlackbox, ztest.SettingPatterns,
			ztest.ProblemMalformedPattern))
		Expect(err).To(MatchError(path.ErrBadPattern))
	})

	It("rejects a malformed helper pattern rather than leaving it matching nothing", func() {
		_, err := ztest.New(map[string]any{
			"blackbox": map[string]any{"helper-patterns": []string{"[helpers_test.go"}},
		})

		Expect(err).To(beConfigError(layout.KindBlackbox, ztest.SettingHelperPatterns,
			ztest.ProblemMalformedPattern))
	})

	It("rejects a malformed adapter pattern rather than leaving it matching nothing", func() {
		_, err := ztest.New(map[string]any{"adapter-patterns": []string{"[suite_test.go"}})

		Expect(err).To(beConfigError(0, ztest.SettingAdapterPatterns, ztest.ProblemMalformedPattern))
	})

	It("rejects a pattern naming the source twice, which it cannot resolve", func() {
		_, err := ztest.New(map[string]any{
			"whitebox": map[string]any{"patterns": []string{"<source>_<source>_test.go"}},
		})

		Expect(err).To(beConfigError(layout.KindWhitebox, ztest.SettingPatterns,
			ztest.ProblemMalformedPattern))
		Expect(err).To(wrapPatternProblem(filepattern.ProblemRepeatedSource))
	})

	It("rejects a glob wrapped around the source token, which is matched literally", func() {
		_, err := ztest.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{"*_<source>_test.go"}},
		})

		Expect(err).To(wrapPatternProblem(filepattern.ProblemGlobAroundSource))
	})

	It("rejects a pattern holding a path, since it matches against a file name", func() {
		_, err := ztest.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{"pkg/<source>_test.go"}},
		})

		Expect(err).To(wrapPatternProblem(filepattern.ProblemPathSeparator))
	})

	It("rejects an empty adapter-patterns, which no file could satisfy", func() {
		_, err := ztest.New(map[string]any{"adapter-patterns": []string{}})

		Expect(err).To(MatchError(&ztest.ConfigError{
			Setting: ztest.SettingAdapterPatterns, Problem: ztest.ProblemNoAdapterName,
		}))
	})

	It("rejects every category being disallowed, which no test file could satisfy", func() {
		_, err := ztest.New(map[string]any{
			"whitebox":   map[string]any{"allowed": false},
			"blackbox":   map[string]any{"allowed": false},
			"standalone": map[string]any{"allowed": false},
		})

		Expect(err).To(MatchError(&ztest.ConfigError{Problem: ztest.ProblemNoKindAllowed}))
	})

	It("accepts tests confined to standalone packages, with neither kind beside a source allowed", func() {
		_, err := ztest.New(map[string]any{
			"whitebox": map[string]any{"allowed": false},
			"blackbox": map[string]any{"allowed": false},
		})

		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects a malformed standalone pattern rather than leaving it matching nothing", func() {
		_, err := ztest.New(map[string]any{
			"standalone": map[string]any{"patterns": []string{"[_test.go"}},
		})

		Expect(err).To(beConfigError(layout.KindStandalone, ztest.SettingPatterns,
			ztest.ProblemMalformedPattern))
	})

	It("rejects the source token in standalone patterns, which name no source", func() {
		_, err := ztest.New(map[string]any{
			"standalone": map[string]any{"patterns": []string{"<source>_test.go"}},
		})

		Expect(err).To(MatchError(&ztest.ConfigError{
			Kind:    layout.KindStandalone,
			Setting: ztest.SettingPatterns,
			Problem: ztest.ProblemPatternNamesSource,
			Pattern: "<source>_test.go",
		}))
	})

	It("rejects the source token in standalone helper patterns, which name no source", func() {
		_, err := ztest.New(map[string]any{
			"standalone": map[string]any{"helper-patterns": []string{"<source>_helpers_test.go"}},
		})

		Expect(err).To(MatchError(&ztest.ConfigError{
			Kind:    layout.KindStandalone,
			Setting: ztest.SettingHelperPatterns,
			Problem: ztest.ProblemPatternNamesSource,
			Pattern: "<source>_helpers_test.go",
		}))
	})

	It("rejects a category allowed under no name at all", func() {
		_, err := ztest.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{}, "helper-patterns": []string{}},
		})

		Expect(err).To(MatchError(&ztest.ConfigError{Kind: layout.KindBlackbox, Problem: ztest.ProblemNoName}))
	})

	It("allows a disallowed category to name nothing, since nothing is what it permits", func() {
		_, err := ztest.New(map[string]any{
			"whitebox": map[string]any{"patterns": []string{}, "helper-patterns": []string{}},
		})

		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects a misspelled spec settings key rather than ignoring it", func() {
		_, err := ztest.New(map[string]any{"build-tag": []string{"integration"}})

		Expect(err).To(beConfigError(0, 0, ztest.ProblemUndecodable))
		Expect(errors.Unwrap(err)).To(MatchError(ContainSubstring(`unknown field "build-tag"`)))
	})

	It("builds with build tags for the api patterns to resolve under", func() {
		_, err := ztest.New(map[string]any{"api": []string{"./..."}, "build-tags": []string{"integration"}})

		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects an explicitly empty api, which would judge no package", func() {
		_, err := ztest.New(map[string]any{"api": []string{}})

		Expect(err).To(beConfigError(0, ztest.SettingAPI, ztest.ProblemEmptyAPI))
	})

	It("builds with an explicit api without resolving it, which is the run's to do", func() {
		chdir(anchorlessDir())

		_, err := ztest.New(map[string]any{"api": []string{"./nowhere/..."}})

		Expect(err).NotTo(HaveOccurred())
	})

	DescribeTable("rejects a rendering pattern the file-name grammar refuses",
		func(pattern string, problem filepattern.Problem) {
			_, err := ztest.New(map[string]any{"rendering-patterns": []string{"<source>_test.go", pattern}})

			Expect(err).To(beConfigError(0, ztest.SettingRenderingPatterns, ztest.ProblemMalformedPattern))
			Expect(err).To(wrapPatternProblem(problem))
		},
		Entry("a path separator", "pkg/<source>_test.go", filepattern.ProblemPathSeparator),
		Entry("a glob around <source>", "*_<source>_test.go", filepattern.ProblemGlobAroundSource),
		Entry("<source> twice", "<source>_<source>_test.go", filepattern.ProblemRepeatedSource),
	)

	DescribeTable("rejects a rendering pattern holding no <source>, which names no source to pin",
		func(pattern string) {
			_, err := ztest.New(map[string]any{"rendering-patterns": []string{pattern}})

			Expect(err).To(And(
				beConfigError(0, ztest.SettingRenderingPatterns, ztest.ProblemPatternNamesNoSource),
				HaveField("Pattern", pattern),
			))
			Expect(errors.Unwrap(err)).To(Succeed())
		},
		Entry("a plain name", "errors_test.go"),
		Entry("a glob", "*_test.go"),
	)

	It("accepts an explicitly empty rendering-patterns, under which no file pins a rendering", func() {
		_, err := ztest.New(map[string]any{"rendering-patterns": []string{}})

		Expect(err).NotTo(HaveOccurred())
	})

	It("exposes exactly the one analyzer golangci-lint should run, named for the linter", func() {
		plugin, err := ztest.New(nil)
		Expect(err).NotTo(HaveOccurred())

		analyzers, err := plugin.BuildAnalyzers()

		Expect(err).NotTo(HaveOccurred())
		Expect(analyzers).To(HaveLen(1))
		Expect(analyzers[0].Name).To(Equal(ztest.Name))
	})

	It("asks for typed loading, which the spec rules resolve names through", func() {
		plugin, err := ztest.New(nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(plugin.GetLoadMode()).To(Equal(register.LoadModeTypesInfo))
	})

	It("hands each half the rules the selection leaves off", func() {
		config, err := ztest.Resolve(ztest.Settings{Disable: []string{"misplaced-fixture", "text-assertion"}})

		Expect(err).NotTo(HaveOccurred())
		Expect(config.Layout.Disabled).To(Equal([]layout.Rule{layout.RuleMisplacedFixture}))
		Expect(config.Spec.Disabled).To(Equal([]spec.Rule{spec.RuleTextAssertion}))
	})

	It("hands the halves every rule under the default", func() {
		config, err := ztest.Resolve(ztest.Settings{})

		Expect(err).NotTo(HaveOccurred())
		Expect(config.Layout.Disabled).To(BeNil())
		Expect(config.Spec.Disabled).To(BeNil())
		Expect(config.Spec.API).To(BeNil())
	})

	It("rejects a rule selection that means nothing, naming the key", func() {
		_, err := ztest.New(map[string]any{"enable": []string{"text-assertion"}})

		Expect(err).To(beConfigError(0, 0, ztest.ProblemRuleSelection))
		Expect(errors.Unwrap(err)).To(MatchError(
			&ruleset.Error{Key: ruleset.KeyEnable, Problem: ruleset.ProblemAlreadyEnabled, Value: "text-assertion"}))
	})

	DescribeTable("rejects a setting whose every rule the selection leaves off",
		func(settings map[string]any, expected *ztest.ConfigError) {
			_, err := ztest.New(settings)

			Expect(err).To(MatchError(expected))
		},
		Entry("a category block without the layout rules",
			map[string]any{"disable": layoutRuleNames(), "whitebox": map[string]any{"allowed": true}},
			&ztest.ConfigError{Kind: layout.KindWhitebox, Problem: ztest.ProblemServesNoRule}),
		Entry("adapter-patterns without the layout rules",
			map[string]any{"disable": layoutRuleNames(), "adapter-patterns": []string{"suite_test.go"}},
			&ztest.ConfigError{Setting: ztest.SettingAdapterPatterns, Problem: ztest.ProblemServesNoRule}),
		Entry("api without unidentified-error",
			map[string]any{"disable": []string{"unidentified-error"}, "api": []string{"./..."}},
			&ztest.ConfigError{Setting: ztest.SettingAPI, Problem: ztest.ProblemServesNoRule}),
		Entry("build-tags without unidentified-error",
			map[string]any{"disable": []string{"unidentified-error"}, "build-tags": []string{"integration"}},
			&ztest.ConfigError{Setting: ztest.SettingBuildTags, Problem: ztest.ProblemServesNoRule}),
		Entry("rendering-patterns without text-assertion or token-assertion",
			map[string]any{
				"disable":            []string{"text-assertion", "token-assertion"},
				"rendering-patterns": []string{"<source>_test.go"},
			},
			&ztest.ConfigError{Setting: ztest.SettingRenderingPatterns, Problem: ztest.ProblemServesNoRule}),
	)
})
