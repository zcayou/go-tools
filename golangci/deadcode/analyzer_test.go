package deadcode_test

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/deadcode"
)

// The fixtures under testdata are their own modules, and the specs run
// the analyzer against passes built the way golangci-lint builds them: one per
// loaded package, with only the syntax the linter asks for.
var _ = Describe("Analyzer", func() {
	It("exposes the analyzer golangci-lint should run, alongside the cache defeat", func() {
		plugin, err := deadcode.New(nil)
		Expect(err).NotTo(HaveOccurred())

		analyzers, err := plugin.BuildAnalyzers()

		Expect(err).NotTo(HaveOccurred())
		Expect(analyzers).To(HaveLen(2))
		Expect(analyzers[0].Name).To(Equal(deadcode.Name))
		Expect(analyzers[0].Doc).NotTo(BeEmpty())
		// golangci-lint folds every analyzer name into its per-package issue cache
		// key, and the name has to differ per process for that cache to miss.
		Expect(analyzers[1].Name).To(HavePrefix(deadcode.Name + "_uncacheable_"))
		Expect(analyzers[1].Name).NotTo(Equal(secondPluginAnalyzerName()))
	})

	It("reports each finding in the pass that holds its file", func() {
		dir := fixtureDir("plain")
		chdir(dir)
		analyzer := newAnalyzer()

		var reported []string
		for _, pkg := range loadFixture(dir, "./...") {
			diagnostics, err := runPass(analyzer, pkg, dir)
			Expect(err).NotTo(HaveOccurred())
			reported = append(reported, diagnostics...)
		}

		Expect(reported).To(ConsistOf(
			"main.go:10:6: unreachable func: Dead",
			"main.go:10:6: unused exported func: Dead",
			"main.go:14:6: test-only unreachable func: Tested",
			"main.go:14:6: test-only unused exported func: Tested",
		))
	})

	It("places a finding in a file a //line directive renames", func() {
		// The engine indexes by the name the loader opened and the pass is looked up
		// by the same name; indexing by the adjusted name reports nothing at all,
		// with no error to show for it.
		dir := fixtureDir("linedirective")
		chdir(dir)
		analyzer := newAnalyzer()

		var reported []string
		for _, pkg := range loadFixture(dir, "./...") {
			diagnostics, err := runPass(analyzer, pkg, dir)
			Expect(err).NotTo(HaveOccurred())
			reported = append(reported, diagnostics...)
		}

		Expect(reported).To(ConsistOf(
			"main.go:8:6: unreachable func: Dead",
			"main.go:8:6: unused exported func: Dead",
		))
	})
})

func fixtureDir(name string) string {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())
	return dir
}

func newAnalyzer() *analysis.Analyzer {
	GinkgoHelper()

	return pluginAnalyzers()[0]
}

func secondPluginAnalyzerName() string {
	GinkgoHelper()

	return pluginAnalyzers()[1].Name
}

func pluginAnalyzers() []*analysis.Analyzer {
	GinkgoHelper()

	plugin, err := deadcode.New(nil)
	Expect(err).NotTo(HaveOccurred())
	analyzers, err := plugin.BuildAnalyzers()
	Expect(err).NotTo(HaveOccurred())
	return analyzers
}

// loadFixture builds the packages a golangci-lint run would hand the analyzer:
// syntax and types, with no dependency graph, since the plugin asks
// for syntax-only loading.
func loadFixture(dir, pattern string) []*packages.Package {
	GinkgoHelper()

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedTypesSizes,
		Dir: dir,
	}, pattern)
	Expect(err).NotTo(HaveOccurred())
	return pkgs
}

// runPass renders what one pass reported. Positions are resolved without //line
// adjustment so that an expectation names the file on disk; golangci-lint
// applies the adjustment itself when it prints.
func runPass(analyzer *analysis.Analyzer, pkg *packages.Package, dir string) ([]string, error) {
	GinkgoHelper()

	var reported []string
	_, err := analyzer.Run(&analysis.Pass{
		Analyzer:   analyzer,
		Fset:       pkg.Fset,
		Files:      pkg.Syntax,
		Pkg:        pkg.Types,
		TypesInfo:  pkg.TypesInfo,
		TypesSizes: pkg.TypesSizes,
		Report: func(diagnostic analysis.Diagnostic) {
			pos := pkg.Fset.PositionFor(diagnostic.Pos, false)
			rel, err := filepath.Rel(dir, pos.Filename)
			Expect(err).NotTo(HaveOccurred())
			reported = append(reported, fmt.Sprintf("%s:%d:%d: %s", rel, pos.Line, pos.Column, diagnostic.Message))
		},
	})
	return reported, err
}
