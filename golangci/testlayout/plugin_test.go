package testlayout_test

import (
	"errors"
	"path"

	"github.com/golangci/plugin-module-register/register"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/zcayou/go-tools/golangci/internal/filepattern"
	"github.com/zcayou/go-tools/golangci/testlayout"
)

var _ = Describe("Plugin", func() {
	It("builds from an absent settings block", func() {
		plugin, err := testlayout.New(nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("builds from an empty settings block", func() {
		plugin, err := testlayout.New(map[string]any{})

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("rejects an unknown settings key rather than ignoring it", func() {
		_, err := testlayout.New(map[string]any{"dedicated-test-dirs": []string{"integration"}})

		Expect(err).To(beConfigError(0, 0, testlayout.ProblemUndecodable))
		Expect(errors.Unwrap(err)).To(MatchError(ContainSubstring(`unknown field "dedicated-test-dirs"`)))
	})

	It("rejects a malformed pattern rather than leaving it matching nothing", func() {
		_, err := testlayout.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{"[_test.go"}},
		})

		Expect(err).To(beConfigError(testlayout.KindBlackbox, testlayout.SettingPatterns,
			testlayout.ProblemMalformedPattern))
		Expect(err).To(MatchError(path.ErrBadPattern))
	})

	It("rejects a malformed helper pattern rather than leaving it matching nothing", func() {
		_, err := testlayout.New(map[string]any{
			"blackbox": map[string]any{"helper-patterns": []string{"[helpers_test.go"}},
		})

		Expect(err).To(beConfigError(testlayout.KindBlackbox, testlayout.SettingHelperPatterns,
			testlayout.ProblemMalformedPattern))
	})

	It("rejects a malformed adapter pattern rather than leaving it matching nothing", func() {
		_, err := testlayout.New(map[string]any{"adapter-patterns": []string{"[suite_test.go"}})

		Expect(err).To(beConfigError(0, testlayout.SettingAdapterPatterns, testlayout.ProblemMalformedPattern))
	})

	It("rejects a pattern naming the source twice, which it cannot resolve", func() {
		_, err := testlayout.New(map[string]any{
			"whitebox": map[string]any{"patterns": []string{"<source>_<source>_test.go"}},
		})

		Expect(err).To(beConfigError(testlayout.KindWhitebox, testlayout.SettingPatterns,
			testlayout.ProblemMalformedPattern))
		Expect(err).To(wrapPatternProblem(filepattern.ProblemRepeatedSource))
	})

	It("rejects a glob wrapped around the source token, which is matched literally", func() {
		_, err := testlayout.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{"*_<source>_test.go"}},
		})

		Expect(err).To(wrapPatternProblem(filepattern.ProblemGlobAroundSource))
	})

	It("rejects a pattern holding a path, since it matches against a file name", func() {
		_, err := testlayout.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{"pkg/<source>_test.go"}},
		})

		Expect(err).To(wrapPatternProblem(filepattern.ProblemPathSeparator))
	})

	It("rejects an empty adapter-patterns, which no file could satisfy", func() {
		_, err := testlayout.New(map[string]any{"adapter-patterns": []string{}})

		Expect(err).To(MatchError(&testlayout.ConfigError{
			Setting: testlayout.SettingAdapterPatterns, Problem: testlayout.ProblemNoAdapterName,
		}))
	})

	It("rejects every category being disallowed, which no test file could satisfy", func() {
		_, err := testlayout.New(map[string]any{
			"whitebox":   map[string]any{"allowed": false},
			"blackbox":   map[string]any{"allowed": false},
			"standalone": map[string]any{"allowed": false},
		})

		Expect(err).To(MatchError(&testlayout.ConfigError{Problem: testlayout.ProblemNoKindAllowed}))
	})

	It("accepts tests confined to standalone packages, with neither kind beside a source allowed", func() {
		_, err := testlayout.New(map[string]any{
			"whitebox": map[string]any{"allowed": false},
			"blackbox": map[string]any{"allowed": false},
		})

		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects a malformed standalone pattern rather than leaving it matching nothing", func() {
		_, err := testlayout.New(map[string]any{
			"standalone": map[string]any{"patterns": []string{"[_test.go"}},
		})

		Expect(err).To(beConfigError(testlayout.KindStandalone, testlayout.SettingPatterns,
			testlayout.ProblemMalformedPattern))
	})

	It("rejects the source token in standalone patterns, which name no source", func() {
		_, err := testlayout.New(map[string]any{
			"standalone": map[string]any{"patterns": []string{"<source>_test.go"}},
		})

		Expect(err).To(MatchError(&testlayout.ConfigError{
			Kind:    testlayout.KindStandalone,
			Setting: testlayout.SettingPatterns,
			Problem: testlayout.ProblemSourcelessPattern,
			Pattern: "<source>_test.go",
		}))
	})

	It("rejects the source token in standalone helper patterns, which name no source", func() {
		_, err := testlayout.New(map[string]any{
			"standalone": map[string]any{"helper-patterns": []string{"<source>_helpers_test.go"}},
		})

		Expect(err).To(MatchError(&testlayout.ConfigError{
			Kind:    testlayout.KindStandalone,
			Setting: testlayout.SettingHelperPatterns,
			Problem: testlayout.ProblemSourcelessPattern,
			Pattern: "<source>_helpers_test.go",
		}))
	})

	It("rejects a category allowed under no name at all", func() {
		_, err := testlayout.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{}, "helper-patterns": []string{}},
		})

		Expect(err).To(MatchError(&testlayout.ConfigError{Kind: testlayout.KindBlackbox, Problem: testlayout.ProblemNoName}))
	})

	It("allows a disallowed category to name nothing, since nothing is what it permits", func() {
		_, err := testlayout.New(map[string]any{
			"whitebox": map[string]any{"patterns": []string{}, "helper-patterns": []string{}},
		})

		Expect(err).NotTo(HaveOccurred())
	})

	It("exposes exactly the analyzer golangci-lint should run", func() {
		plugin, err := testlayout.New(nil)
		Expect(err).NotTo(HaveOccurred())

		analyzers, err := plugin.BuildAnalyzers()

		Expect(err).NotTo(HaveOccurred())
		Expect(analyzers).To(HaveLen(1))
		Expect(analyzers[0].Name).To(Equal(testlayout.Name))
	})

	It("asks for syntax-only loading", func() {
		plugin, err := testlayout.New(nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(plugin.GetLoadMode()).To(Equal(register.LoadModeSyntax))
	})
})

// beConfigError matches a [testlayout.ConfigError] at the given place,
// whatever cause it carries.
func beConfigError(kind testlayout.Kind, setting testlayout.Setting, problem testlayout.Problem) types.GomegaMatcher {
	return SatisfyAll(
		BeAssignableToTypeOf(&testlayout.ConfigError{}),
		HaveField("Kind", kind),
		HaveField("Setting", setting),
		HaveField("Problem", problem),
	)
}

// wrapPatternProblem matches an error carrying a [filepattern.Error]
// for the given problem.
func wrapPatternProblem(problem filepattern.Problem) types.GomegaMatcher {
	return WithTransform(func(err error) filepattern.Problem {
		patternErr, ok := errors.AsType[*filepattern.Error](err)
		if !ok {
			return 0
		}
		return patternErr.Problem
	}, Equal(problem))
}
