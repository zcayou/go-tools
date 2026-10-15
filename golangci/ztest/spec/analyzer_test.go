package spec_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/zcayou/go-tools/golangci/ztest"
	"github.com/zcayou/go-tools/golangci/ztest/spec"
)

var _ = Describe("Analyzer", func() {
	It("names each rule by the token its messages open with", func() {
		Expect(spec.RuleUnidentifiedError.String()).To(Equal("unidentified-error"))
		Expect(spec.RuleTextAssertion.String()).To(Equal("text-assertion"))
		Expect(spec.RuleTokenAssertion.String()).To(Equal("token-assertion"))
		Expect(spec.RuleBackgroundContext.String()).To(Equal("background-context"))

		var unset spec.Rule
		Expect(unset.String()).To(Equal("Rule(0)"))
	})

	It("reports HaveOccurred and a negated Succeed under every polarity an assertion gives them", func() {
		runAnalysis(ztest.Settings{}, "polarity")
	})

	It("resolves Gomega however a file imports it, and takes nothing else for Gomega's", func() {
		runAnalysis(ztest.Settings{}, "importforms")
	})

	It("reports testify's Error through the package, a dot import, an *Assertions value and a suite's receiver", func() {
		runAnalysis(ztest.Settings{}, "testifyforms")
	})

	It("judges unidentified-error only in the specs of API packages", func() {
		runAnalysis(ztest.Settings{}, "internalcase/internal/thing", "mainpkg")
	})

	It("reports each Gomega form of asserting text, but not a negated one or one on a cause", func() {
		runAnalysis(ztest.Settings{}, "textforms")
	})

	It("reports each testify form of asserting text, but not a Not assertion or one on a cause", func() {
		runAnalysis(ztest.Settings{}, "testifytext")
	})

	It("reports each assertion on an enum's rendering, known by the fact its package exports", func() {
		runAnalysis(ztest.Settings{}, "tokenforms/states", "tokenforms")
	})

	It("lets the test files named for the files declaring an enum and its String pin its tokens", func() {
		runAnalysis(ztest.Settings{}, "tokenpinning")
	})

	It("lets the test file named for a source declaring Error pin its rendering", func() {
		runAnalysis(ztest.Settings{}, "pinning")
	})

	It("says nothing about a file named for a source that will not parse, which could declare Error", func() {
		runAnalysis(ztest.Settings{}, "unparsable")
	})

	It("honors configured rendering patterns in place of the default", func() {
		runAnalysis(ztest.Settings{RenderingPatterns: []string{"<source>_rendering_test.go"}}, "customrendering")
	})

	It("reports context.Background and context.TODO in a test, however imported, but not where no handle runs", func() {
		runAnalysis(ztest.Settings{}, "backgroundcontext")
	})

	It("lets no file pin a rendering under an empty rendering-patterns", func() {
		runAnalysis(ztest.Settings{RenderingPatterns: []string{}}, "emptyrendering")
	})
})

// runAnalysis drives the analyzer over testdata/src/<pattern> and asserts
// the diagnostics against the `want` comments in those files.
func runAnalysis(settings ztest.Settings, patterns ...string) {
	GinkgoHelper()

	config, err := ztest.Resolve(settings)
	Expect(err).NotTo(HaveOccurred())

	analysistest.Run(GinkgoT(), analysistest.TestData(), spec.NewAnalyzer(config.Spec), patterns...)
}
