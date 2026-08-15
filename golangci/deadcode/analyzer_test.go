package deadcode_test

import (
	"fmt"
	"os"
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

	It("declares no fact types, which would force analysis of every dependency", func() {
		plugin, err := deadcode.New(nil)
		Expect(err).NotTo(HaveOccurred())

		analyzers, err := plugin.BuildAnalyzers()
		Expect(err).NotTo(HaveOccurred())

		Expect(analyzers[0].FactTypes).To(BeEmpty())
		Expect(analyzers[0].Requires).To(BeEmpty())
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

	It("drops a finding in a file no pass holds", func() {
		// Reporting is scoped to what golangci-lint was asked to lint, while
		// the analysis always covers the module: the finding in main.go has nowhere
		// to go.
		dir := fixtureDir("plain")
		chdir(dir)
		analyzer := newAnalyzer()

		var reported []string
		for _, pkg := range loadFixture(dir, "./sub") {
			diagnostics, err := runPass(analyzer, pkg, dir)
			Expect(err).NotTo(HaveOccurred())
			reported = append(reported, diagnostics...)
		}

		Expect(reported).To(BeEmpty())
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

	It("shares one analysis across every pass", func() {
		// The second pass runs from a directory the engine could not anchor at, so
		// it can only succeed on a result the first pass already produced.
		dir := fixtureDir("plain")
		chdir(dir)
		analyzer := newAnalyzer()
		pkgs := loadFixture(dir, "./...")
		Expect(pkgs).To(HaveLen(2))

		_, err := runPass(analyzer, pkgs[0], dir)
		Expect(err).NotTo(HaveOccurred())

		chdir(anchorlessDir())
		_, err = runPass(analyzer, pkgs[1], dir)

		Expect(err).NotTo(HaveOccurred())
	})

	It("fails every pass when the analysis cannot run", func() {
		// golangci-lint keeps the first error and skips caching on any error, so
		// there is nothing to be gained by reporting the failure once and staying
		// silent afterwards.
		dir := fixtureDir("plain")
		pkgs := loadFixture(dir, "./...")
		chdir(anchorlessDir())
		analyzer := newAnalyzer()

		_, first := runPass(analyzer, pkgs[0], dir)
		_, second := runPass(analyzer, pkgs[1], dir)

		Expect(first).To(MatchError(ContainSubstring("no go.mod at or above")))
		Expect(second).To(MatchError(first))
	})
})

func fixtureDir(name string) string {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())
	return dir
}

// anchorlessDir returns a directory with no go.mod at or above it, which
// is what a repository golangci-lint cannot anchor looks like from the plugin.
func anchorlessDir() string {
	GinkgoHelper()

	dir, err := os.MkdirTemp("", "deadcode-anchorless")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { Expect(os.RemoveAll(dir)).To(Succeed()) })
	resolved, err := filepath.EvalSymlinks(dir)
	Expect(err).NotTo(HaveOccurred())
	Expect(filepath.Join(resolved, "go.mod")).NotTo(BeAnExistingFile())
	return resolved
}

func chdir(dir string) {
	GinkgoHelper()

	previous, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chdir(dir)).To(Succeed())
	DeferCleanup(func() { Expect(os.Chdir(previous)).To(Succeed()) })
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
