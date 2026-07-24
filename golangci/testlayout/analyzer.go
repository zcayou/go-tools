package testlayout

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Rule names the convention a diagnostic came from.
//
// golangci-lint discards [analysis.Diagnostic.Category], so the rule is also
// the first word of every message; that is what makes it addressable
// by a linters.exclusions text rule.
type Rule string

const (
	// RuleTestPackage fires on a test file of a kind the settings do not allow:
	// a white-box file where only black-box files may be, or the reverse.
	RuleTestPackage Rule = "test-package"

	// RuleTestFileName fires on a test file whose name no pattern for its kind
	// allows — most often a spec file with no source file to be named after.
	RuleTestFileName Rule = "test-file-name"

	// RuleSpecLessTestFile fires on a file named for the source it exercises
	// that carries no specs, so the specs for that source are somewhere else.
	RuleSpecLessTestFile Rule = "spec-less-test-file"

	// RuleSpecsInHelperFile fires on a file named for its supporting role
	// that carries specs anyway, which puts specs where a reader is not looking.
	RuleSpecsInHelperFile Rule = "specs-in-helper-file"

	// RuleGinkgoAdapterFile fires where RunSpecs is called from a file
	// the settings do not reserve for the adapter.
	RuleGinkgoAdapterFile Rule = "ginkgo-adapter-file"

	// RuleGinkgoAdapterMissing fires on a package registering Ginkgo specs where
	// no file the directory's test binary compiles calls RunSpecs, so none
	// of those specs run.
	RuleGinkgoAdapterMissing Rule = "ginkgo-adapter-missing"

	// RuleGinkgoAdapterDuplicate fires on a second call to RunSpecs in one test
	// binary, which Ginkgo rejects at run time.
	RuleGinkgoAdapterDuplicate Rule = "ginkgo-adapter-duplicate"

	// RuleSuiteFileContents fires on anything a suite file declares beyond the go
	// test entry point and the suite-level hooks.
	RuleSuiteFileContents Rule = "suite-file-contents"
)

const testSuffix = "_test.go"

const analyzerDoc = `checks that test files follow the repository's test-suite layout conventions

A test file is white-box or black-box depending on whether it declares the
package under test or the external <pkg>_test package, and each kind is allowed
or not and named after a configured set of patterns. A file named for a source
file carries the specs for it; a file named for a supporting role carries no
specs. Where Ginkgo is used, the adapter that calls RunSpecs sits in a file
reserved for it, and it exists.`

// NewAnalyzer builds the analyzer that enforces the layout conventions under
// settings. It fails on settings no file could satisfy.
func NewAnalyzer(settings Settings) (*analysis.Analyzer, error) {
	config, err := settings.resolve()
	if err != nil {
		return nil, err
	}
	c := &checker{config: config}

	return &analysis.Analyzer{
		Name: Name,
		Doc:  analyzerDoc,
		Run:  c.run,
	}, nil
}

type checker struct {
	config config
}

func (c *checker) run(pass *analysis.Pass) (any, error) {
	own := testFiles(pass)
	if len(own) == 0 {
		return nil, nil
	}

	// Every file of a package sits in one directory, so the first is as good
	// as any for naming it.
	dir, err := readDirectory(filepath.Dir(own[0].path))
	if err != nil {
		return nil, err
	}

	for _, file := range own {
		if !c.checkFile(pass, file, dir.sources) {
			continue
		}
		c.checkAdapterFile(pass, file, dir.sources)
		c.checkSuiteFile(pass, file, dir.sources)
	}
	return nil, c.checkAdapterExists(pass, own, dir)
}

// checkFile reports what one test file's kind and name violate, and reports
// whether anything more is worth saying about the file. A file of a kind
// that may not exist is reported once and left there: neither the name
// it should have had nor what it holds says anything useful about a file
// that should not be present.
func (c *checker) checkFile(pass *analysis.Pass, file testFile, srcs sources) bool {
	category := c.config.category(file.blackbox)

	if !category.allowed {
		report(pass, file.syntax.Package, RuleTestPackage, fmt.Sprintf(
			"%s test files are not allowed; declare package %s",
			category.name, otherPackage(file.syntax.Name.Name)))
		return false
	}

	// Helper patterns are consulted first, so a helpers.go sitting beside
	// helpers_test.go does not turn it into a file owing specs.
	switch {
	case category.helpers.matches(file.name, srcs):
		if file.info.specs {
			report(pass, file.syntax.Package, RuleSpecsInHelperFile,
				"a file named for its supporting role carries specs; "+category.specsBelong())
		}
	case category.specs.matches(file.name, srcs):
		if !file.info.specs {
			report(pass, file.syntax.Package, RuleSpecLessTestFile,
				"no specs or test functions; "+category.helpersBelong())
		}
	default:
		report(pass, file.syntax.Package, RuleTestFileName, fmt.Sprintf(
			"not a name a %s test file may have; want %s", category.name, category.names()))
	}
	return true
}

// checkAdapterFile reports an adapter outside the files reserved for it.
// It does not consult whether the package uses Ginkgo: a call to RunSpecs
// is Ginkgo.
func (c *checker) checkAdapterFile(pass *analysis.Pass, file testFile, srcs sources) {
	if !file.info.adapter.IsValid() || c.config.adapters.matches(file.name, srcs) {
		return
	}
	report(pass, file.info.adapter, RuleGinkgoAdapterFile,
		"the Ginkgo adapter belongs in "+c.config.adapters.String())
}

// checkSuiteFile reports what a suite file declares beyond its reason
// for existing. The file hands the package's specs to go test: the entry point,
// and the hooks that bracket the whole binary. A helper here is out
// of the place a reader looks for it, and a spec here registers against
// the very suite the file is meant to start.
func (c *checker) checkSuiteFile(pass *analysis.Pass, file testFile, srcs sources) {
	if !c.config.adapters.matches(file.name, srcs) {
		return
	}
	for _, decl := range file.syntax.Decls {
		if what, allowed := suiteDecl(decl); !allowed {
			report(pass, decl.Pos(), RuleSuiteFileContents, what+
				"; a suite file holds the test entry point and the suite-level hooks, nothing else")
		}
	}
}

// checkAdapterExists reports a package that registers Ginkgo specs with nothing
// to run them.
//
// The adapter is one per test binary rather than one per package, so the whole
// directory is judged, and only the pass holding the file the diagnostic lands
// on reports it. A test file no build compiles is no more part of the binary
// than a file in another directory, so it neither supplies an adapter nor
// demands one. What a custom run.build-tags would add is out of reach, because
// a plugin cannot read that setting.
func (c *checker) checkAdapterExists(pass *analysis.Pass, own []testFile, dir directory) error {
	held := make(map[string]testFile, len(own))
	for _, file := range own {
		held[file.name] = file
	}

	// The first spec-bearing file anchors a missing-adapter diagnostic,
	// and the first adapter is the one a duplicate is measured against. Both
	// passes covering the directory walk the same sorted names, so both settle
	// on the same files and each reports only what it holds.
	var adapters []string
	var anchor string
	for _, name := range dir.tests {
		file, ours := held[name]
		found := file.info
		if !ours {
			switch compiled, err := build.Default.MatchFile(dir.path, name); {
			case err != nil:
				return fmt.Errorf("reading %s: %w", name, err)
			case !compiled:
				continue
			}
			parsed, err := inspectPath(filepath.Join(dir.path, name))
			switch {
			case errors.Is(err, errUnparsed):
				return nil
			case err != nil:
				return err
			}
			found = parsed
		}

		if found.adapter.IsValid() {
			adapters = append(adapters, name)
		}
		if found.ginkgo && anchor == "" {
			anchor = name
		}
	}

	switch {
	case len(adapters) > 1:
		// The first adapter in name order is the one that stays, so the extras
		// are what a reader has to delete.
		for _, name := range adapters[1:] {
			if file, ours := held[name]; ours {
				report(pass, file.info.adapter, RuleGinkgoAdapterDuplicate,
					"a second file calls "+runSpecs+"; one test binary runs one suite, so "+
						adapters[0]+" already runs these specs")
			}
		}

	case len(adapters) == 0 && anchor != "":
		if file, ours := held[anchor]; ours {
			report(pass, file.syntax.Package, RuleGinkgoAdapterMissing,
				"this package registers Ginkgo specs but no file calls "+runSpecs+
					", so none of them run; add "+c.config.adapters.String())
		}
	}
	return nil
}

// testFile is one *_test.go file a pass holds.
type testFile struct {
	syntax   *ast.File
	path     string
	name     string
	blackbox bool
	info     info
}

func testFiles(pass *analysis.Pass) []testFile {
	var files []testFile
	for _, syntax := range pass.Files {
		path := pass.Fset.Position(syntax.Package).Filename
		if !strings.HasSuffix(path, testSuffix) {
			continue
		}
		files = append(files, testFile{
			syntax:   syntax,
			path:     path,
			name:     filepath.Base(path),
			blackbox: strings.HasSuffix(syntax.Name.Name, "_test"),
			info:     inspect(syntax),
		})
	}
	return files
}

// otherPackage names the package a test file of the other kind would declare,
// so a diagnostic can say what to write rather than only what not to.
func otherPackage(name string) string {
	if base, ok := strings.CutSuffix(name, "_test"); ok {
		return base
	}
	return name + "_test"
}

func report(pass *analysis.Pass, pos token.Pos, rule Rule, message string) {
	pass.Report(analysis.Diagnostic{
		Pos:      pos,
		Category: string(rule),
		Message:  string(rule) + ": " + message,
	})
}
