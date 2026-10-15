package layout_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/zcayou/go-tools/golangci/ztest"
	"github.com/zcayou/go-tools/golangci/ztest/layout"
)

var _ = Describe("Analyzer", func() {
	It("accepts specs beside their source, an adapter in suite_test.go, and spec-less helpers", func() {
		runAnalysis(ztest.Settings{}, "conventional")
	})

	It("reports a test file declaring the package under test", func() {
		runAnalysis(ztest.Settings{}, "whitebox")
	})

	It("reports an unmatched name, a spec-less spec file, and specs in a helper file", func() {
		runAnalysis(ztest.Settings{}, "badnames")
	})

	It("reports a file whose only source is a sibling that shares its prefix", func() {
		runAnalysis(ztest.Settings{}, "sharedprefix")
	})

	It("accepts a container builder as the only spec a file carries", func() {
		runAnalysis(ztest.Settings{}, "builders")
	})

	It("recognizes Ginkgo however a file imports it", func() {
		runAnalysis(ztest.Settings{}, "ginkgoimports")
	})

	It("takes no call for Ginkgo's that does not go through an import of it", func() {
		runAnalysis(ztest.Settings{}, "lookalikes")
	})

	It("reports a Ginkgo adapter outside the files reserved for it", func() {
		runAnalysis(ztest.Settings{}, "adapterfile")
	})

	It("reports a package whose Ginkgo specs nothing runs", func() {
		runAnalysis(ztest.Settings{}, "adaptermissing")
	})

	It("reports a second adapter, which leaves one test binary running two suites", func() {
		runAnalysis(ztest.Settings{
			AdapterPatterns: []string{"gadget_test.go", "suite_test.go"},
		}, "adapterduplicate")
	})

	It("reports an adapter in doc_test.go, which reserves suite_test.go for one", func() {
		runAnalysis(ztest.Settings{}, "docsuite")
	})

	It("reports what a suite file declares beyond the entry point and the suite hooks", func() {
		runAnalysisPastSkippedPackage(ztest.Settings{}, "suitecontents")
	})

	It("accepts a whitebox test file where the category is allowed", func() {
		runAnalysis(ztest.Settings{
			Whitebox: &ztest.Category{Allowed: new(true)},
		}, "whiteboxallowed")
	})

	It("reports a blackbox test file where only whitebox files may be", func() {
		runAnalysis(ztest.Settings{
			Whitebox: &ztest.Category{Allowed: new(true)},
			Blackbox: &ztest.Category{Allowed: new(false)},
		}, "blackboxdenied")
	})

	It("says nothing more about a file of a kind that may not exist", func() {
		runAnalysis(ztest.Settings{}, "disallowedextras")
	})

	It("honors a configured name pattern in place of the default", func() {
		runAnalysis(ztest.Settings{
			Blackbox: &ztest.Category{
				Patterns:       []string{"spec_<source>_test.go"},
				HelperPatterns: []string{},
			},
		}, "customnames")
	})

	It("honors configured whitebox patterns in place of the defaults", func() {
		runAnalysis(ztest.Settings{
			Whitebox: &ztest.Category{
				Allowed:        new(true),
				Patterns:       []string{"<source>_internal_test.go"},
				HelperPatterns: []string{"internal_helpers_test.go"},
			},
		}, "whiteboxcustom")
	})

	It("honors configured adapter names in place of suite_test.go", func() {
		runAnalysis(ztest.Settings{
			Blackbox: &ztest.Category{
				Patterns: []string{"<source>_test.go", "harness_test.go"},
			},
			AdapterPatterns: []string{"harness_test.go"},
		}, "customadapter")
	})

	It("consults helper patterns before spec patterns, for a name matching both", func() {
		runAnalysis(ztest.Settings{}, "helperwins")
	})

	It("names the setting when no spec-bearing name is allowed", func() {
		runAnalysis(ztest.Settings{
			Blackbox: &ztest.Category{Patterns: []string{}},
		}, "nospecnames")
	})

	It("names the setting when no helper name is allowed", func() {
		runAnalysis(ztest.Settings{
			Blackbox: &ztest.Category{HelperPatterns: []string{}},
		}, "nohelpernames")
	})

	It("finds an adapter the other test package of the directory holds", func() {
		runAnalysis(ztest.Settings{
			Whitebox: &ztest.Category{Allowed: new(true)},
		}, "mixed")
	})

	It("reports a missing adapter once where both test packages register specs", func() {
		runAnalysis(ztest.Settings{
			Whitebox: &ztest.Category{Allowed: new(true)},
		}, "mixedbad")
	})

	It("takes no adapter from a test file no build compiles", func() {
		runAnalysis(ztest.Settings{}, "ignoredfiles")
	})

	It("says nothing about a directory holding a test file that will not parse", func() {
		runAnalysisPastSkippedPackage(ztest.Settings{
			Whitebox: &ztest.Category{Allowed: new(true)},
		}, "unparsable")
	})

	It("accepts scenario-named specs in a directory holding no source", func() {
		runAnalysis(ztest.Settings{}, "standalone")
	})

	It("keeps a directory standalone under a doc.go that declares nothing", func() {
		runAnalysis(ztest.Settings{}, "standalonedoc")
	})

	It("reports every test file of a standalone directory where the kind may not exist", func() {
		runAnalysis(ztest.Settings{
			Standalone: &ztest.Category{Allowed: new(false)},
		}, "standalonedenied")
	})

	It("holds a standalone directory to its own name patterns and to the adapter rules", func() {
		runAnalysis(ztest.Settings{
			Standalone: &ztest.Category{Patterns: []string{"*_integration_test.go"}},
		}, "standalonenames")
	})

	It("reports a spec file's fixture that another file consumes, and says where it belongs", func() {
		runAnalysis(ztest.Settings{}, "misplacedfixture")
	})

	It("takes no name for a fixture's where a scope declares it afresh, or where it keys a struct", func() {
		runAnalysis(ztest.Settings{}, "fixturescopes")
	})

	It("reports a helper file serving one file, crediting what it serves through other helpers", func() {
		runAnalysis(ztest.Settings{
			Blackbox: &ztest.Category{HelperPatterns: []string{"*_helpers_test.go"}},
		}, "fixturehelpers")
	})

	It("counts a black-box file reaching into the white-box package, which cannot hold what it uses", func() {
		runAnalysis(ztest.Settings{
			Whitebox: &ztest.Category{
				Allowed:        new(true),
				Patterns:       []string{"<source>_internal_test.go"},
				HelperPatterns: []string{"export_test.go"},
			},
		}, "fixtureboundary")
	})

	It("counts consumers under every build, not only the one the pass covers", func() {
		runAnalysis(ztest.Settings{}, "fixturebuilds")
	})

	It("says nothing about fixtures beside a test file that will not parse", func() {
		runAnalysisPastSkippedPackage(ztest.Settings{
			Whitebox: &ztest.Category{Allowed: new(true)},
		}, "fixtureunparsable")
	})

	It("names the setting when no helper file may hold a shared fixture", func() {
		runAnalysis(ztest.Settings{
			Blackbox: &ztest.Category{HelperPatterns: []string{}},
		}, "fixturenohelpers")
	})
})

// runAnalysis drives the analyzer over testdata/src/<pattern> and asserts
// the diagnostics against the `want` comments in those files.
func runAnalysis(settings ztest.Settings, pattern string) {
	GinkgoHelper()

	runAnalysisAs(GinkgoT(), settings, pattern)
}

// runAnalysisPastSkippedPackage is runAnalysis for a fixture the go tool cannot
// build a test binary for, which is what it takes to put a file that will not
// parse, or an entry point over a signature go test would not run, in front
// of the linter. The driver declines to run the pass covering such a package
// and analysistest calls that a failure; every other pass still runs, and every
// `want` comment in the files those passes hold is still asserted.
func runAnalysisPastSkippedPackage(settings ztest.Settings, pattern string) {
	GinkgoHelper()

	runAnalysisAs(toleratesSkippedPackage{GinkgoT()}, settings, pattern)
}

func runAnalysisAs(t analysistest.Testing, settings ztest.Settings, pattern string) {
	GinkgoHelper()

	config, err := ztest.Resolve(settings)
	Expect(err).NotTo(HaveOccurred())

	analysistest.Run(t, analysistest.TestData(), layout.NewAnalyzer(config.Layout), pattern)
}

const skippedPackage = "analysis skipped due to errors in package"

type toleratesSkippedPackage struct{ analysistest.Testing }

func (t toleratesSkippedPackage) Errorf(format string, args ...any) {
	if strings.Contains(fmt.Sprintf(format, args...), skippedPackage) {
		return
	}
	t.Testing.Errorf(format, args...)
}
