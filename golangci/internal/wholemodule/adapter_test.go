package wholemodule_test

import (
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// The specs run the analyzer against passes built the way golangci-lint builds
// them: one per loaded package, with only the syntax the linter asks for.
var _ = Describe("Linter", func() {
	It("returns the analyzer that reports, alongside the cache defeat", func() {
		analyzers := wholemodule.New("example", "reports examples", nil).Analyzers()

		Expect(analyzers).To(HaveLen(2))
		Expect(analyzers[0].Name).To(Equal("example"))
		Expect(analyzers[0].Doc).To(Equal("reports examples"))
		// golangci-lint folds every analyzer name into its per-package issue cache
		// key, and the name has to differ per process for that cache to miss.
		Expect(analyzers[1].Name).To(HavePrefix("example_uncacheable_"))
		Expect(analyzers[1].Name).NotTo(Equal(wholemodule.New("example", "", nil).Analyzers()[1].Name))
	})

	It("declares no fact types, which would force analysis of every dependency", func() {
		analyzers := wholemodule.New("example", "", nil).Analyzers()

		Expect(analyzers[0].FactTypes).To(BeEmpty())
		Expect(analyzers[0].Requires).To(BeEmpty())
	})

	It("reports each finding in the pass that holds its file, under its category", func() {
		dir := fixtureDir("plain")
		chdir(dir)
		analyzer := analyzerFinding(
			findingAt(dir, "lib/lib.go", "func Spread", "rule: Spread"),
			findingAt(dir, "main.go", "func main", "rule: main"),
		)

		Expect(runPasses(analyzer, dir, "./...")).To(ConsistOf(
			"lib/lib.go:9:1: [rule] rule: Spread",
			"main.go:5:1: [rule] rule: main",
		))
	})

	It("drops a finding in a file no pass holds", func() {
		// Reporting is scoped to what golangci-lint was asked to lint, while
		// the analysis always covers the module: the finding in main.go has nowhere
		// to go.
		dir := fixtureDir("plain")
		chdir(dir)
		analyzer := analyzerFinding(findingAt(dir, "main.go", "func main", "rule: main"))

		Expect(runPasses(analyzer, dir, "./lib")).To(BeEmpty())
	})

	It("drops a finding past the end of its file, which changed underneath the run", func() {
		dir := fixtureDir("plain")
		chdir(dir)
		finding := findingAt(dir, "main.go", "func main", "rule: main")
		finding.Pos.Offset = 1 << 20

		Expect(runPasses(analyzerFinding(finding), dir, "./...")).To(BeEmpty())
	})

	It("analyzes the module once, from its root, however many passes run", func() {
		dir := fixtureDir("plain")
		chdir(filepath.Join(dir, "lib"))
		var roots []string
		analyzer := wholemodule.New("example", "", func(root string) ([]wholemodule.Finding, error) {
			roots = append(roots, root)
			return nil, nil
		}).Analyzers()[0]

		runPasses(analyzer, dir, "./...")

		Expect(roots).To(Equal([]string{dir}))
	})

	It("fails every pass when the analysis cannot run", func() {
		// golangci-lint keeps the first error and skips caching on any error, so
		// there is nothing to be gained by reporting the failure once and staying
		// silent afterwards.
		dir := fixtureDir("plain")
		pkgs := loadPasses(dir, "./...")
		chdir(anchorlessDir())
		analyzer := wholemodule.New("example", "", nil).Analyzers()[0]

		_, first := runPass(analyzer, pkgs[0], dir)
		_, second := runPass(analyzer, pkgs[1], dir)

		Expect(first).To(BeAssignableToTypeOf(&wholemodule.NoModuleError{}))
		Expect(second).To(MatchError(first))
	})

	It("fails every pass with the analysis's own error", func() {
		dir := fixtureDir("plain")
		chdir(dir)
		failure := errors.New("analysis failed")
		analyzer := wholemodule.New("example", "", func(string) ([]wholemodule.Finding, error) {
			return nil, failure
		}).Analyzers()[0]

		for _, pkg := range loadPasses(dir, "./...") {
			_, err := runPass(analyzer, pkg, dir)
			Expect(err).To(MatchError(failure))
		}
	})
})

func analyzerFinding(findings ...wholemodule.Finding) *analysis.Analyzer {
	return wholemodule.New("example", "", func(string) ([]wholemodule.Finding, error) {
		return findings, nil
	}).Analyzers()[0]
}

// findingAt places a finding at the first occurrence of text in the fixture
// file rel, categorized by the rule its message opens with.
func findingAt(dir, rel, text, message string) wholemodule.Finding {
	GinkgoHelper()

	path := filepath.Join(dir, rel)
	content, err := os.ReadFile(path) //nolint:gosec // the path is a fixture file the spec names
	Expect(err).NotTo(HaveOccurred())
	offset := strings.Index(string(content), text)
	Expect(offset).NotTo(BeNumerically("<", 0), "%s holds no %q", rel, text)
	category, _, _ := strings.Cut(message, ":")
	return wholemodule.Finding{
		Pos:      token.Position{Filename: path, Offset: offset},
		Category: category,
		Message:  message,
	}
}

// loadPasses builds the packages a golangci-lint run would hand the analyzer:
// syntax and types, with no dependency graph, since a whole-module linter asks
// for syntax-only loading.
func loadPasses(dir, pattern string) []*packages.Package {
	GinkgoHelper()

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedTypesSizes,
		Dir: dir,
	}, pattern)
	Expect(err).NotTo(HaveOccurred())
	return pkgs
}

func runPasses(analyzer *analysis.Analyzer, dir, pattern string) []string {
	GinkgoHelper()

	var reported []string
	for _, pkg := range loadPasses(dir, pattern) {
		diagnostics, err := runPass(analyzer, pkg, dir)
		Expect(err).NotTo(HaveOccurred())
		reported = append(reported, diagnostics...)
	}
	return reported
}

// runPass renders what one pass reported, as file:line:column: [category]
// message.
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
			reported = append(reported, fmt.Sprintf("%s:%d:%d: [%s] %s",
				rel, pos.Line, pos.Column, diagnostic.Category, diagnostic.Message))
		},
	})
	return reported, err
}
