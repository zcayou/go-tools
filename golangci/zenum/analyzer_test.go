package zenum_test

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/zenum"
	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

// The fixture under testdata is its own module, and the specs run the analyzer
// against passes built the way golangci-lint builds them: one per loaded
// package, with only the syntax the linter asks for.
var _ = Describe("Analyzer", func() {
	It("exposes the analyzer golangci-lint should run, alongside the cache defeat", func() {
		analyzers, err := newPlugin(nil).BuildAnalyzers()

		Expect(err).NotTo(HaveOccurred())
		Expect(analyzers).To(HaveLen(2))
		Expect(analyzers[0].Name).To(Equal(zenum.Name))
		Expect(analyzers[0].Doc).NotTo(BeEmpty())
		Expect(analyzers[1].Name).To(HavePrefix(zenum.Name + "_uncacheable_"))
	})

	It("reports each finding in the pass that holds its file, opening with its rule", func() {
		dir := fixtureDir()
		chdir(dir)
		analyzer := firstAnalyzer(nil)

		var reported []string
		for _, pkg := range loadFixture(dir) {
			diagnostics, err := runPass(analyzer, pkg, dir)
			Expect(err).NotTo(HaveOccurred())
			reported = append(reported, diagnostics...)
		}

		Expect(reported).To(ConsistOf(
			"kinds/kinds.go:6:2: zero-member: zero-member: KindNone is kinds.Kind's zero value, which an unset field "+
				"also holds; start the members above zero",
			"main.go:10:21: numeric-verb: numeric-verb: formats kinds.Kind under %d, which prints its ordinal rather "+
				"than its member's name; use %v or %s",
		))
	})

	It("fails the run on an exception that suppresses nothing", func() {
		dir := fixtureDir()
		chdir(dir)
		analyzer := firstAnalyzer(map[string]any{
			"zero-defaults": []any{map[string]any{"type": "example.com/plain/kinds.Missing", "reason": "documented"}},
		})

		_, err := runPass(analyzer, loadFixture(dir)[0], dir)

		Expect(err).To(MatchError(&engine.ExceptionError{
			Setting: engine.SettingZeroDefaults, Fault: engine.FaultNotEnum, Type: "example.com/plain/kinds.Missing",
		}))
	})
})

func fixtureDir() string {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join("testdata", "plain"))
	Expect(err).NotTo(HaveOccurred())
	return dir
}

func firstAnalyzer(settings any) *analysis.Analyzer {
	GinkgoHelper()

	analyzers, err := newPlugin(settings).BuildAnalyzers()
	Expect(err).NotTo(HaveOccurred())
	return analyzers[0]
}

// loadFixture builds the packages a golangci-lint run would hand the analyzer:
// syntax and types, with no dependency graph, since the plugin asks
// for syntax-only loading.
func loadFixture(dir string) []*packages.Package {
	GinkgoHelper()

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedTypesSizes,
		Dir: dir,
	}, "./...")
	Expect(err).NotTo(HaveOccurred())
	return pkgs
}

// runPass renders what one pass reported, with the diagnostic's category
// ahead of its message.
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
			reported = append(reported, fmt.Sprintf("%s:%d:%d: %s: %s",
				rel, pos.Line, pos.Column, diagnostic.Category, diagnostic.Message))
		},
	})
	return reported, err
}
