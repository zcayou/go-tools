package testlayout_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/zcayou/go-tools/golangci/testlayout"
)

var _ = Describe("Analyzer", func() {
	It("is named for the linter", func() {
		analyzer, err := testlayout.NewAnalyzer(testlayout.Settings{})

		Expect(err).NotTo(HaveOccurred())
		Expect(analyzer.Name).To(Equal(testlayout.Name))
	})

	It("accepts specs beside their source, an adapter in suite_test.go, and spec-less helpers", func() {
		runAnalysis(testlayout.Settings{}, "conventional")
	})

	It("reports a test file declaring the package under test", func() {
		runAnalysis(testlayout.Settings{}, "whitebox")
	})

	It("reports an unmatched name, a spec-less spec file, and specs in a helper file", func() {
		runAnalysis(testlayout.Settings{}, "badnames")
	})

	It("reports a file whose only source is a sibling that shares its prefix", func() {
		runAnalysis(testlayout.Settings{}, "sharedprefix")
	})

	It("accepts a container builder as the only spec a file carries", func() {
		runAnalysis(testlayout.Settings{}, "builders")
	})

	It("recognizes Ginkgo however a file imports it", func() {
		runAnalysis(testlayout.Settings{}, "ginkgoimports")
	})

	It("takes no call for Ginkgo's that does not go through an import of it", func() {
		runAnalysis(testlayout.Settings{}, "lookalikes")
	})

	It("reports a Ginkgo adapter outside the files reserved for it", func() {
		runAnalysis(testlayout.Settings{}, "adapterfile")
	})

	It("reports a package whose Ginkgo specs nothing runs", func() {
		runAnalysis(testlayout.Settings{}, "adaptermissing")
	})

	It("reports a second adapter, which leaves one test binary running two suites", func() {
		runAnalysis(testlayout.Settings{
			AdapterPatterns: []string{"gadget_test.go", "suite_test.go"},
		}, "adapterduplicate")
	})

	It("reports an adapter in doc_test.go, which reserves suite_test.go for one", func() {
		runAnalysis(testlayout.Settings{}, "docsuite")
	})

	It("reports what a suite file declares beyond the entry point and the suite hooks", func() {
		runAnalysisPastSkippedPackage(testlayout.Settings{}, "suitecontents")
	})

	It("accepts a whitebox test file where the category is allowed", func() {
		runAnalysis(testlayout.Settings{
			Whitebox: &testlayout.Category{Allowed: new(true)},
		}, "whiteboxallowed")
	})

	It("reports a blackbox test file where only whitebox files may be", func() {
		runAnalysis(testlayout.Settings{
			Whitebox: &testlayout.Category{Allowed: new(true)},
			Blackbox: &testlayout.Category{Allowed: new(false)},
		}, "blackboxdenied")
	})

	It("says nothing more about a file of a kind that may not exist", func() {
		runAnalysis(testlayout.Settings{}, "disallowedextras")
	})

	It("honors a configured name pattern in place of the default", func() {
		runAnalysis(testlayout.Settings{
			Blackbox: &testlayout.Category{
				Patterns:       []string{"spec_<source>_test.go"},
				HelperPatterns: []string{},
			},
		}, "customnames")
	})

	It("honors configured whitebox patterns in place of the defaults", func() {
		runAnalysis(testlayout.Settings{
			Whitebox: &testlayout.Category{
				Allowed:        new(true),
				Patterns:       []string{"<source>_internal_test.go"},
				HelperPatterns: []string{"internal_helpers_test.go"},
			},
		}, "whiteboxcustom")
	})

	It("honors configured adapter names in place of suite_test.go", func() {
		runAnalysis(testlayout.Settings{
			Blackbox: &testlayout.Category{
				Patterns: []string{"<source>_test.go", "harness_test.go"},
			},
			AdapterPatterns: []string{"harness_test.go"},
		}, "customadapter")
	})

	It("consults helper patterns before spec patterns, for a name matching both", func() {
		runAnalysis(testlayout.Settings{}, "helperwins")
	})

	It("names the setting when no spec-bearing name is allowed", func() {
		runAnalysis(testlayout.Settings{
			Blackbox: &testlayout.Category{Patterns: []string{}},
		}, "nospecnames")
	})

	It("names the setting when no helper name is allowed", func() {
		runAnalysis(testlayout.Settings{
			Blackbox: &testlayout.Category{HelperPatterns: []string{}},
		}, "nohelpernames")
	})

	It("finds an adapter the other test package of the directory holds", func() {
		runAnalysis(testlayout.Settings{
			Whitebox: &testlayout.Category{Allowed: new(true)},
		}, "mixed")
	})

	It("reports a missing adapter once where both test packages register specs", func() {
		runAnalysis(testlayout.Settings{
			Whitebox: &testlayout.Category{Allowed: new(true)},
		}, "mixedbad")
	})

	It("takes no adapter from a test file no build compiles", func() {
		runAnalysis(testlayout.Settings{}, "ignoredfiles")
	})

	It("says nothing about a directory holding a test file that will not parse", func() {
		runAnalysisPastSkippedPackage(testlayout.Settings{
			Whitebox: &testlayout.Category{Allowed: new(true)},
		}, "unparsable")
	})

	It("accepts scenario-named specs in a directory holding no source", func() {
		runAnalysis(testlayout.Settings{}, "standalone")
	})

	It("keeps a directory standalone under a doc.go that declares nothing", func() {
		runAnalysis(testlayout.Settings{}, "standalonedoc")
	})

	It("reports every test file of a standalone directory where the kind may not exist", func() {
		runAnalysis(testlayout.Settings{
			Standalone: &testlayout.Category{Allowed: new(false)},
		}, "standalonedenied")
	})

	It("holds a standalone directory to its own name patterns and to the adapter rules", func() {
		runAnalysis(testlayout.Settings{
			Standalone: &testlayout.Category{Patterns: []string{"*_integration_test.go"}},
		}, "standalonenames")
	})
})

// runAnalysis drives the analyzer over testdata/src/<pattern> and asserts
// the diagnostics against the `want` comments in those files.
func runAnalysis(settings testlayout.Settings, pattern string) {
	GinkgoHelper()

	runAnalysisAs(GinkgoT(), settings, pattern)
}

// runAnalysisPastSkippedPackage is runAnalysis for a fixture the go tool cannot
// build a test binary for, which is what it takes to put a file that will not
// parse, or an entry point over a signature go test would not run, in front
// of the linter. The driver declines to run the pass covering such a package
// and analysistest calls that a failure; every other pass still runs, and every
// `want` comment in the files those passes hold is still asserted.
func runAnalysisPastSkippedPackage(settings testlayout.Settings, pattern string) {
	GinkgoHelper()

	runAnalysisAs(toleratesSkippedPackage{GinkgoT()}, settings, pattern)
}

func runAnalysisAs(t analysistest.Testing, settings testlayout.Settings, pattern string) {
	GinkgoHelper()

	analyzer, err := testlayout.NewAnalyzer(settings)
	Expect(err).NotTo(HaveOccurred())

	analysistest.Run(t, analysistest.TestData(), analyzer, pattern)
}

const skippedPackage = "analysis skipped due to errors in package"

type toleratesSkippedPackage struct{ analysistest.Testing }

func (t toleratesSkippedPackage) Errorf(format string, args ...any) {
	if strings.Contains(fmt.Sprintf(format, args...), skippedPackage) {
		return
	}
	t.Testing.Errorf(format, args...)
}
