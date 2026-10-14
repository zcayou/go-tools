package zassert_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/zassert"
)

// beConfigError matches the configuration error zassert returns for problem
// with setting.
func beConfigError(setting zassert.Setting, problem zassert.Problem) types.GomegaMatcher {
	return And(
		BeAssignableToTypeOf(&zassert.ConfigError{}),
		HaveField("Setting", setting),
		HaveField("Problem", problem),
	)
}

// apiscopeDir is the module fixture the explicit api setting is resolved in.
func apiscopeDir() string {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join("testdata", "apiscope"))
	Expect(err).NotTo(HaveOccurred())
	return dir
}

// anchorlessDir returns a directory with no go.mod at or above it, which
// is what a repository golangci-lint cannot anchor looks like from the plugin.
func anchorlessDir() string {
	GinkgoHelper()

	dir, err := os.MkdirTemp("", "zassert-anchorless")
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

// loadPasses builds the packages a golangci-lint run with run.tests on would
// hand the analyzer, test packages included. The fixture is a module of its
// own, which an explicit api setting is resolved against.
func loadPasses(dir string) []*packages.Package {
	GinkgoHelper()

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedTypesSizes,
		Dir:   dir,
		Tests: true,
	}, "./...")
	Expect(err).NotTo(HaveOccurred())
	return pkgs
}

// runPass renders what one pass reported, as file:line: message, with the file
// relative to dir.
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
			pos := pkg.Fset.Position(diagnostic.Pos)
			rel, err := filepath.Rel(dir, pos.Filename)
			Expect(err).NotTo(HaveOccurred())
			reported = append(reported, fmt.Sprintf("%s:%d: %s", rel, pos.Line, diagnostic.Message))
		},
	})
	return reported, err
}
