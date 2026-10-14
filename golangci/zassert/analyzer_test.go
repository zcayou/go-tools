package zassert_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/zcayou/go-tools/golangci/zassert"
)

var _ = Describe("Analyzer", func() {
	It("is named for the linter", func() {
		analyzer, err := zassert.NewAnalyzer(zassert.Settings{})

		Expect(err).NotTo(HaveOccurred())
		Expect(analyzer.Name).To(Equal(zassert.Name))
	})

	It("names each rule by the token its messages open with", func() {
		Expect(zassert.RuleUnidentifiedError.String()).To(Equal("unidentified-error"))
		Expect(zassert.RuleTextAssertion.String()).To(Equal("text-assertion"))
		Expect(zassert.RuleTokenAssertion.String()).To(Equal("token-assertion"))

		var unset zassert.Rule
		Expect(unset.String()).To(Equal("Rule(0)"))
	})

	It("reports HaveOccurred and a negated Succeed under every polarity an assertion gives them", func() {
		runAnalysis(zassert.Settings{}, "polarity")
	})

	It("resolves Gomega however a file imports it, and takes nothing else for Gomega's", func() {
		runAnalysis(zassert.Settings{}, "importforms")
	})

	It("reports testify's Error through the package, a dot import, an *Assertions value and a suite's receiver", func() {
		runAnalysis(zassert.Settings{}, "testifyforms")
	})

	It("judges unidentified-error only in the specs of API packages", func() {
		runAnalysis(zassert.Settings{}, "internalcase/internal/thing", "mainpkg")
	})

	It("reports each Gomega form of asserting text, but not a negated one or one on a cause", func() {
		runAnalysis(zassert.Settings{}, "textforms")
	})

	It("reports each testify form of asserting text, but not a Not assertion or one on a cause", func() {
		runAnalysis(zassert.Settings{}, "testifytext")
	})

	It("reports each assertion on an enum's rendering, known by the fact its package exports", func() {
		runAnalysis(zassert.Settings{}, "tokenforms/states", "tokenforms")
	})

	It("lets the test files named for the files declaring an enum and its String pin its tokens", func() {
		runAnalysis(zassert.Settings{}, "tokenpinning")
	})

	It("lets the test file named for a source declaring Error pin its rendering", func() {
		runAnalysis(zassert.Settings{}, "pinning")
	})

	It("says nothing about a file named for a source that will not parse, which could declare Error", func() {
		runAnalysis(zassert.Settings{}, "unparsable")
	})

	It("honors configured rendering patterns in place of the default", func() {
		runAnalysis(zassert.Settings{RenderingPatterns: []string{"<source>_rendering_test.go"}}, "customrendering")
	})

	It("lets no file pin a rendering under an empty rendering-patterns", func() {
		runAnalysis(zassert.Settings{RenderingPatterns: []string{}}, "emptyrendering")
	})
})

// runAnalysis drives the analyzer over testdata/src/<pattern> and asserts
// the diagnostics against the `want` comments in those files.
func runAnalysis(settings zassert.Settings, patterns ...string) {
	GinkgoHelper()

	analyzer, err := zassert.NewAnalyzer(settings)
	Expect(err).NotTo(HaveOccurred())

	analysistest.Run(GinkgoT(), analysistest.TestData(), analyzer, patterns...)
}
