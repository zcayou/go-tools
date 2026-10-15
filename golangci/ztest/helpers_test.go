package ztest_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/ztest"
	"github.com/zcayou/go-tools/golangci/ztest/internal/filepattern"
	"github.com/zcayou/go-tools/golangci/ztest/layout"
)

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

	dir, err := os.MkdirTemp("", "ztest-anchorless")
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

// beConfigError matches a [ztest.ConfigError] at the given place, whatever
// cause it carries.
func beConfigError(kind layout.Kind, setting ztest.Setting, problem ztest.Problem) types.GomegaMatcher {
	return SatisfyAll(
		BeAssignableToTypeOf(&ztest.ConfigError{}),
		HaveField("Kind", kind),
		HaveField("Setting", setting),
		HaveField("Problem", problem),
	)
}

// wrapPatternProblem matches an error carrying a [filepattern.Error]
// for the given problem.
func wrapPatternProblem(problem filepattern.Problem) types.GomegaMatcher {
	return WithTransform(func(err error) filepattern.Problem {
		patternErr, ok := errors.AsType[*filepattern.Error](err)
		if !ok {
			return 0
		}
		return patternErr.Problem
	}, Equal(problem))
}

// layoutRuleNames are the tokens of every layout rule, which a selection
// disables to leave the spec rules alone.
func layoutRuleNames() []string {
	rules := []layout.Rule{
		layout.RuleTestPackage, layout.RuleTestFileName, layout.RuleSpecLessTestFile, layout.RuleSpecsInHelperFile,
		layout.RuleGinkgoAdapterFile, layout.RuleGinkgoAdapterMissing, layout.RuleGinkgoAdapterDuplicate,
		layout.RuleSuiteFileContents, layout.RuleMisplacedFixture, layout.RuleSingleConsumerHelperFile,
	}
	names := make([]string, len(rules))
	for i, rule := range rules {
		names[i] = rule.String()
	}
	return names
}
