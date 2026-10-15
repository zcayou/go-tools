package layout

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/ztest/internal/filepattern"
)

// Rule names the convention a diagnostic came from.
//
// golangci-lint discards [analysis.Diagnostic.Category], so the rule is also
// the first word of every message; that is what makes it addressable
// by a linters.exclusions text rule.
type Rule int

const (
	// RuleTestPackage fires on a test file of a kind the settings do not allow:
	// a white-box file where only black-box files may be, or the reverse,
	// or a standalone file where every test file sits beside a source.
	RuleTestPackage Rule = iota + 1

	// RuleTestFileName fires on a test file whose name no pattern for its kind
	// allows — most often a spec file with no source file to be named after.
	RuleTestFileName

	// RuleSpecLessTestFile fires on a file named for the source it exercises
	// that carries no specs, so the specs for that source are somewhere else.
	RuleSpecLessTestFile

	// RuleSpecsInHelperFile fires on a file named for its supporting role
	// that carries specs anyway, which puts specs where a reader is not looking.
	RuleSpecsInHelperFile

	// RuleGinkgoAdapterFile fires where RunSpecs is called from a file
	// the settings do not reserve for the adapter.
	RuleGinkgoAdapterFile

	// RuleGinkgoAdapterMissing fires on a package registering Ginkgo specs where
	// no file the directory's test binary compiles calls RunSpecs, so none
	// of those specs run.
	RuleGinkgoAdapterMissing

	// RuleGinkgoAdapterDuplicate fires on a second call to RunSpecs in one test
	// binary, which Ginkgo rejects at run time.
	RuleGinkgoAdapterDuplicate

	// RuleSuiteFileContents fires on anything a suite file declares beyond the go
	// test entry point and the suite-level hooks.
	RuleSuiteFileContents

	// RuleMisplacedFixture fires on a fixture a spec file declares that another
	// file consumes, which a reader of that file finds only by searching.
	RuleMisplacedFixture

	// RuleSingleConsumerHelperFile fires on a helper file whose every fixture
	// serves one file, which a name promising shared fixtures should not hold.
	RuleSingleConsumerHelperFile
)

// String is the rule's token, which opens every message the rule reports.
func (r Rule) String() string {
	switch r {
	case RuleTestPackage:
		return "test-package"
	case RuleTestFileName:
		return "test-file-name"
	case RuleSpecLessTestFile:
		return "spec-less-test-file"
	case RuleSpecsInHelperFile:
		return "specs-in-helper-file"
	case RuleGinkgoAdapterFile:
		return "ginkgo-adapter-file"
	case RuleGinkgoAdapterMissing:
		return "ginkgo-adapter-missing"
	case RuleGinkgoAdapterDuplicate:
		return "ginkgo-adapter-duplicate"
	case RuleSuiteFileContents:
		return "suite-file-contents"
	case RuleMisplacedFixture:
		return "misplaced-fixture"
	case RuleSingleConsumerHelperFile:
		return "single-consumer-helper-file"
	default:
		return "Rule(" + strconv.Itoa(int(r)) + ")"
	}
}

const testSuffix = "_test.go"

const analyzerDoc = `checks that test files follow the repository's test-suite layout conventions

A test file is white-box or black-box depending on whether it declares the
package under test or the external <pkg>_test package, or standalone where its
directory holds no source to test; each kind is allowed or not and named after
a configured set of patterns. A file named for a source file carries the specs
for it; a file named for a supporting role carries no specs. Where Ginkgo is
used, the adapter that calls RunSpecs sits in a file reserved for it, and it
exists. A fixture lives in the one file consuming it, or in a helper file when
more than one does.`

// NewAnalyzer builds the analyzer that enforces the layout conventions under
// config.
func NewAnalyzer(config Config) *analysis.Analyzer {
	c := &checker{config: config}

	return &analysis.Analyzer{
		Name: "layout",
		Doc:  analyzerDoc,
		Run:  c.run,
	}
}

type checker struct {
	config Config
}

func (c *checker) run(pass *analysis.Pass) (any, error) {
	own := testFiles(pass)
	if len(own) == 0 {
		return nil, nil
	}

	// Every file of a package sits in one directory, so the first is as good
	// as any for naming it. A source file that will not parse leaves
	// the directory's kind unknown, and the driver has already reported it.
	dir, err := readDirectory(filepath.Dir(own[0].path))
	switch {
	case errors.Is(err, errUnparsed):
		return nil, nil
	case err != nil:
		return nil, err
	}

	siblings, err := readSiblings(dir, own)
	if err != nil {
		return nil, err
	}

	for _, file := range own {
		if !c.checkFile(pass, file, dir) {
			continue
		}
		c.checkAdapterFile(pass, file, dir.sources)
		c.checkSuiteFile(pass, file, dir.sources)
	}
	c.checkFixtures(pass, own, siblings, dir)
	return nil, c.checkAdapterExists(pass, own, siblings, dir)
}

// checkFile reports what one test file's kind and name violate, and reports
// whether anything more is worth saying about the file. A file of a kind
// that may not exist is reported once and left there: neither the name
// it should have had nor what it holds says anything useful about a file
// that should not be present.
func (c *checker) checkFile(pass *analysis.Pass, file testFile, dir directory) bool {
	category := c.config.category(file, dir)

	if !category.Allowed {
		c.report(pass, file.syntax.Package, RuleTestPackage, fmt.Sprintf(
			"%s test files are not allowed; %s", category.Kind, category.forbidden(file.syntax.Name.Name)))
		return false
	}

	// Helper patterns are consulted first, so a helpers.go sitting beside
	// helpers_test.go does not turn it into a file owing specs.
	switch {
	case category.Helpers.Matches(file.name, dir.sources):
		if file.info.specs {
			c.report(pass, file.syntax.Package, RuleSpecsInHelperFile,
				"a file named for its supporting role carries specs; "+category.specsBelong())
		}
	case category.Specs.Matches(file.name, dir.sources):
		if !file.info.specs {
			c.report(pass, file.syntax.Package, RuleSpecLessTestFile,
				"no specs or test functions; "+category.helpersBelong())
		}
	default:
		c.report(pass, file.syntax.Package, RuleTestFileName, fmt.Sprintf(
			"not a name a %s test file may have; want %s", category.Kind, category.names()))
	}
	return true
}

// checkAdapterFile reports an adapter outside the files reserved for it.
// It does not consult whether the package uses Ginkgo: a call to RunSpecs
// is Ginkgo.
func (c *checker) checkAdapterFile(pass *analysis.Pass, file testFile, srcs filepattern.Sources) {
	if !file.info.adapter.IsValid() || c.config.Adapters.Matches(file.name, srcs) {
		return
	}
	c.report(pass, file.info.adapter, RuleGinkgoAdapterFile,
		"the Ginkgo adapter belongs in "+c.config.Adapters.String())
}

// checkSuiteFile reports what a suite file declares beyond its reason
// for existing. The file hands the package's specs to go test: the entry point,
// and the hooks that bracket the whole binary. A helper here is out
// of the place a reader looks for it, and a spec here registers against
// the very suite the file is meant to start.
func (c *checker) checkSuiteFile(pass *analysis.Pass, file testFile, srcs filepattern.Sources) {
	if !c.config.Adapters.Matches(file.name, srcs) {
		return
	}
	imports := ginkgoImportsOf(file.syntax)
	for _, decl := range file.syntax.Decls {
		if what, allowed := suiteDecl(decl, imports); !allowed {
			c.report(pass, decl.Pos(), RuleSuiteFileContents, what+
				"; a suite file holds the test entry point and the suite-level hooks, nothing else")
		}
	}
}

// checkFixtures reports fixtures held away from the files that consume them:
// one in a spec file that another file consumes, and a helper file serving only
// one file. Every test file of the directory counts as a consumer, whatever
// its build constraints, since where a fixture lives is a property
// of the source tree rather than of one build; a file that will not parse
// could consume anything, so nothing is said.
func (c *checker) checkFixtures(pass *analysis.Pass, own []testFile, siblings siblings, dir directory) {
	if len(siblings.unparsed) > 0 {
		return
	}

	files := slices.Concat(own, slices.Collect(maps.Values(siblings.files)))
	byName := make(map[string]testFile, len(files))
	for _, file := range files {
		byName[file.name] = file
	}

	var sources []*ast.File
	for _, syntax := range pass.Files {
		if !strings.HasSuffix(pass.Fset.Position(syntax.Package).Filename, testSuffix) {
			sources = append(sources, syntax)
		}
	}
	underTest := pass.Pkg.Path()
	if strings.HasSuffix(pass.Pkg.Name(), "_test") {
		underTest = strings.TrimSuffix(underTest, "_test")
	}
	graph := readFixtures(files, sources, underTest)

	// canHold reports whether the file named consumer could declare what from
	// declares: it shares from's package, and it is not a suite file, which
	// holds nothing beyond its entry point and hooks.
	canHold := func(consumer string, from testFile) bool {
		file := byName[consumer]
		return file.syntax.Name.Name == from.syntax.Name.Name && c.role(file, dir) != roleSuite
	}

	for _, file := range own {
		category := c.config.category(file, dir)
		if !category.Allowed {
			continue
		}
		switch c.role(file, dir) {
		case roleSpec:
			for _, f := range graph.byFile[file.name] {
				consumers := sortedKeys(f.consumers)
				switch {
				case len(consumers) == 0 || slices.Equal(consumers, []string{file.name}):
				case len(consumers) == 1 && canHold(consumers[0], file):
					c.report(pass, f.ident.Pos(), RuleMisplacedFixture, fmt.Sprintf(
						"%s is used only by %s; declare it there", f.name, f.consumerNames(consumers)))
				default:
					c.report(pass, f.ident.Pos(), RuleMisplacedFixture, fmt.Sprintf(
						"%s is used by %s; %s", f.name, f.consumerNames(consumers), category.fixturesBelong()))
				}
			}

		case roleHelper:
			served := make(map[string]bool)
			for _, f := range graph.byFile[file.name] {
				maps.Copy(served, f.consumers)
			}
			delete(served, file.name)
			if consumers := sortedKeys(served); len(consumers) == 1 && canHold(consumers[0], file) {
				c.report(pass, file.syntax.Package, RuleSingleConsumerHelperFile, fmt.Sprintf(
					"everything %s declares serves only %s; move it there", file.name, consumers[0]))
			}

		case roleSuite, roleOther:
			// suite-file-contents already judges what a suite file declares, and a file
			// no pattern names has its name to answer for first.
		}
	}
}

// role is what a test file's name says it holds, for the rules judging where
// fixtures live.
type role int

const (
	// roleOther is a file no pattern names. It draws test-file-name,
	// and the fixture rules count it as a consumer without judging it.
	roleOther role = iota

	// roleHelper is a file named for a supporting role.
	roleHelper

	// roleSuite is a file reserved for the Ginkgo adapter.
	roleSuite

	// roleSpec is a file named for what it exercises.
	roleSpec
)

// role reads a file's role off its name. Helper patterns are consulted first,
// as [checker.checkFile] consults them, and a suite file matching a spec
// pattern, as suite_test.go does by default, is a suite file.
func (c *checker) role(file testFile, dir directory) role {
	category := c.config.category(file, dir)
	switch {
	case category.Helpers.Matches(file.name, dir.sources):
		return roleHelper
	case c.config.Adapters.Matches(file.name, dir.sources):
		return roleSuite
	case category.Specs.Matches(file.name, dir.sources):
		return roleSpec
	}
	return roleOther
}

func sortedKeys(set map[string]bool) []string {
	return slices.Sorted(maps.Keys(set))
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
func (c *checker) checkAdapterExists(pass *analysis.Pass, own []testFile, siblings siblings, dir directory) error {
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
			case siblings.unparsed[name]:
				return nil
			}
			found = siblings.files[name].info
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
				c.report(pass, file.info.adapter, RuleGinkgoAdapterDuplicate,
					"a second file calls "+runSpecs+"; one test binary runs one suite, so "+
						adapters[0]+" already runs these specs")
			}
		}

	case len(adapters) == 0 && anchor != "":
		if file, ours := held[anchor]; ours {
			c.report(pass, file.syntax.Package, RuleGinkgoAdapterMissing,
				"this package registers Ginkgo specs but no file calls "+runSpecs+
					", so none of them run; add "+c.config.Adapters.String())
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
		files = append(files, newTestFile(syntax, path))
	}
	return files
}

func newTestFile(syntax *ast.File, path string) testFile {
	return testFile{
		syntax:   syntax,
		path:     path,
		name:     filepath.Base(path),
		blackbox: strings.HasSuffix(syntax.Name.Name, "_test"),
		info:     inspect(syntax),
	}
}

// siblings are the test files of a directory that a pass does not hold:
// the other test package's, and those of builds other than the one the pass
// covers. They are read from disk, whatever their build constraints, and each
// rule decides which of them it counts.
type siblings struct {
	files map[string]testFile

	// unparsed names the files that will not parse. What such a file holds
	// is unknown, so a rule it could answer for says nothing.
	unparsed map[string]bool
}

func readSiblings(dir directory, own []testFile) (siblings, error) {
	read := siblings{files: make(map[string]testFile), unparsed: make(map[string]bool)}
	for _, name := range dir.tests {
		if slices.ContainsFunc(own, func(file testFile) bool { return file.name == name }) {
			continue
		}
		path := filepath.Join(dir.path, name)
		syntax, err := parsePath(path)
		switch {
		case errors.Is(err, errUnparsed):
			read.unparsed[name] = true
		case err != nil:
			return siblings{}, err
		default:
			read.files[name] = newTestFile(syntax, path)
		}
	}
	return read, nil
}

// otherPackage names the package a test file of the other kind would declare,
// so a diagnostic can say what to write rather than only what not to.
func otherPackage(name string) string {
	if base, ok := strings.CutSuffix(name, "_test"); ok {
		return base
	}
	return name + "_test"
}

// report reports a finding of rule, unless the configuration disables it.
func (c *checker) report(pass *analysis.Pass, pos token.Pos, rule Rule, message string) {
	if slices.Contains(c.config.Disabled, rule) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      pos,
		Category: rule.String(),
		Message:  rule.String() + ": " + message,
	})
}
