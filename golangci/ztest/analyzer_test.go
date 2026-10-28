package ztest_test

import (
	"errors"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/ztest"
)

// An explicit api setting is resolved against the module root, which
// analysistest's GOPATH layout has none of, so these specs drive the analyzer
// over a fixture module the way golangci-lint would: one pass per loaded
// package, test packages included.
var _ = Describe("API scope", func() {
	It("judges unidentified-error only in the specs of the packages an explicit api names", func() {
		Expect(runFixture(ztest.Settings{API: []string{"./narrow"}})).To(ConsistOf(
			"narrow/narrow_test.go:12: unidentified-error: HaveOccurred says only that some error occurred; " +
				"assert which one, by its type and fields",
		))
	})

	It("takes no internal package for API, whatever pattern matches it", func() {
		Expect(runFixture(ztest.Settings{API: []string{"./..."}})).To(ConsistOf(
			HavePrefix("narrow/narrow_test.go:12: unidentified-error: "),
			HavePrefix("wide/wide_test.go:12: unidentified-error: "),
		))
	})

	It("fails the run on a pattern matching no package", func() {
		err := runFixtureError(ztest.Settings{API: []string{"./narrow", "./empty/..."}})

		Expect(err).To(And(
			beConfigError(0, ztest.SettingAPI, ztest.ProblemUnresolvedAPIPattern),
			HaveField("Pattern", "./empty/..."),
		))
		cause, ok := errors.AsType[*wholemodule.PatternError](err)
		Expect(ok).To(BeTrue())
		Expect(cause.Excluded).To(BeEmpty())
	})

	It("resolves the api patterns under the configured build tags", func() {
		Expect(runFixture(ztest.Settings{
			API: []string{"./narrow", "./tagged"}, BuildTags: []string{"ztesttagged"},
		})).To(ConsistOf(HavePrefix("narrow/narrow_test.go:12: unidentified-error: ")))
	})

	It("fails the run on a pattern only the build tags it is not given would match", func() {
		err := runFixtureError(ztest.Settings{API: []string{"./narrow", "./tagged"}})

		Expect(err).To(And(
			beConfigError(0, ztest.SettingAPI, ztest.ProblemUnresolvedAPIPattern),
			HaveField("Pattern", "./tagged"),
		))
	})

	It("fails the run on a pattern matching only internal packages", func() {
		err := runFixtureError(ztest.Settings{API: []string{"./internal/..."}})

		Expect(err).To(And(
			beConfigError(0, ztest.SettingAPI, ztest.ProblemUnresolvedAPIPattern),
			HaveField("Pattern", "./internal/..."),
		))
		cause, ok := errors.AsType[*wholemodule.PatternError](err)
		Expect(ok).To(BeTrue())
		Expect(cause.Excluded).To(ConsistOf("example.com/apiscope/internal/hidden"))
	})

	It("fails the run where no module root anchors an explicit api", func() {
		dir := apiscopeDir()
		pkgs := loadPasses(dir)
		analyzer, err := ztest.NewAnalyzer(ztest.Settings{API: []string{"./narrow"}})
		Expect(err).NotTo(HaveOccurred())
		chdir(anchorlessDir())

		_, err = runPass(analyzer, pkgs[0], dir)

		Expect(err).To(beConfigError(0, ztest.SettingAPI, ztest.ProblemNoModuleRoot))
		_, ok := errors.AsType[*wholemodule.NoModuleError](err)
		Expect(ok).To(BeTrue())
	})

	It("needs no module root under the default", func() {
		dir := apiscopeDir()
		pkgs := loadPasses(dir)
		analyzer, err := ztest.NewAnalyzer(ztest.Settings{})
		Expect(err).NotTo(HaveOccurred())
		chdir(anchorlessDir())

		var reported []string
		for _, pkg := range pkgs {
			diagnostics, err := runPass(analyzer, pkg, dir)
			Expect(err).NotTo(HaveOccurred())
			reported = append(reported, diagnostics...)
		}

		Expect(reported).To(ConsistOf(
			HavePrefix("narrow/narrow_test.go:12: unidentified-error: "),
			HavePrefix("wide/wide_test.go:12: unidentified-error: "),
		))
	})
})

var _ = Describe("Analyzer", func() {
	It("reports the layout rules' findings under its own name", func() {
		analysistest.Run(GinkgoT(), halfTestData("layout"), newAnalyzer(ztest.Settings{}), "badnames")
	})

	It("reports the spec rules' findings, and nothing from rules the selection leaves off", func() {
		analysistest.Run(GinkgoT(), halfTestData("spec"),
			newAnalyzer(ztest.Settings{Disable: layoutRuleNames()}), "polarity")
	})

	It("declares the spec rules' facts only while a spec rule runs", func() {
		Expect(newAnalyzer(ztest.Settings{}).FactTypes).NotTo(BeEmpty())
		Expect(newAnalyzer(ztest.Settings{
			Disable: []string{
				"unidentified-error", "text-assertion", "token-assertion", "background-context", "boolean-identity",
			},
		}).FactTypes).To(BeEmpty())
	})
})

// halfTestData is the analysistest tree a half's own specs run against.
func halfTestData(half string) string {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join(half, "testdata"))
	Expect(err).NotTo(HaveOccurred())
	return dir
}

// newAnalyzer builds the analyzer under settings the spec expects it to take.
func newAnalyzer(settings ztest.Settings) *analysis.Analyzer {
	GinkgoHelper()

	analyzer, err := ztest.NewAnalyzer(settings)
	Expect(err).NotTo(HaveOccurred())
	return analyzer
}

// runFixture runs the analyzer over every package of the apiscope fixture
// from its root, and returns what the passes reported.
func runFixture(settings ztest.Settings) []string {
	GinkgoHelper()

	dir := apiscopeDir()
	chdir(dir)
	analyzer, err := ztest.NewAnalyzer(settings)
	Expect(err).NotTo(HaveOccurred())

	var reported []string
	for _, pkg := range loadPasses(dir) {
		diagnostics, err := runPass(analyzer, pkg, dir)
		Expect(err).NotTo(HaveOccurred())
		reported = append(reported, diagnostics...)
	}
	return reported
}

// runFixtureError runs the analyzer over the apiscope fixture from its root,
// expecting every pass to fail alike, and returns the error.
func runFixtureError(settings ztest.Settings) error {
	GinkgoHelper()

	dir := apiscopeDir()
	chdir(dir)
	analyzer, err := ztest.NewAnalyzer(settings)
	Expect(err).NotTo(HaveOccurred())

	var first error
	for _, pkg := range loadPasses(dir) {
		_, err := runPass(analyzer, pkg, dir)
		Expect(err).To(BeAssignableToTypeOf(&ztest.ConfigError{}))
		if first == nil {
			first = err
		}
		Expect(err).To(BeIdenticalTo(first))
	}
	return first
}
