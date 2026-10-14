package zerrors_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/zerrors"
)

// runFixture runs a fresh plugin's analyzer over every package of a fixture
// module, as golangci-lint would from inside it, and renders what each pass
// reported. It returns the first error a pass returned.
func runFixture(name string) ([]string, error) {
	GinkgoHelper()

	dir := fixtureDir(name)
	chdir(dir)
	analyzer := newAnalyzer()

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax,
		Dir:  dir,
	}, "./...")
	Expect(err).NotTo(HaveOccurred())

	var reported []string
	for _, pkg := range pkgs {
		_, err := analyzer.Run(&analysis.Pass{
			Analyzer: analyzer,
			Fset:     pkg.Fset,
			Files:    pkg.Syntax,
			Report: func(diagnostic analysis.Diagnostic) {
				pos := pkg.Fset.PositionFor(diagnostic.Pos, false)
				rel, err := filepath.Rel(dir, pos.Filename)
				Expect(err).NotTo(HaveOccurred())
				reported = append(reported, fmt.Sprintf("%s:%d:%d: [%s] %s",
					rel, pos.Line, pos.Column, diagnostic.Category, diagnostic.Message))
			},
		})
		if err != nil {
			return nil, err
		}
	}
	return reported, nil
}

func newAnalyzer() *analysis.Analyzer {
	GinkgoHelper()

	plugin, err := zerrors.New(nil)
	Expect(err).NotTo(HaveOccurred())
	analyzers, err := plugin.BuildAnalyzers()
	Expect(err).NotTo(HaveOccurred())
	return analyzers[0]
}

func fixtureDir(name string) string {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())
	return dir
}

func chdir(dir string) {
	GinkgoHelper()

	previous, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chdir(dir)).To(Succeed())
	DeferCleanup(func() { Expect(os.Chdir(previous)).To(Succeed()) })
}
