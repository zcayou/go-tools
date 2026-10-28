package spec

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Rule names the convention a diagnostic came from.
//
// golangci-lint discards [analysis.Diagnostic.Category], so the rule is also
// the first word of every message; that is what makes it addressable
// by a linters.exclusions text rule.
type Rule int

const (
	// RuleUnidentifiedError fires on an assertion that an error occurred which
	// says nothing about which error, in the specs of an API package.
	RuleUnidentifiedError Rule = iota + 1

	// RuleTextAssertion fires on a positive assertion on an error's text,
	// in a test file that does not pin that rendering.
	RuleTextAssertion

	// RuleTokenAssertion fires on a positive assertion on an enum's rendering,
	// in a test file that does not pin that enum's rendering.
	RuleTokenAssertion

	// RuleBackgroundContext fires on context.Background or context.TODO in a test
	// file, where the test framework hands each node a context of its own.
	RuleBackgroundContext

	// RuleBooleanIdentity fires on an identity boolean, what errors.Is
	// or errors.As reported, asserted as a boolean where the framework has
	// an assertion that reports the error.
	RuleBooleanIdentity

	// RuleUnpinnedRendering fires on an exported error type whose rendering no
	// test file pins.
	RuleUnpinnedRendering
)

func (r Rule) String() string {
	switch r {
	case RuleUnidentifiedError:
		return "unidentified-error"
	case RuleTextAssertion:
		return "text-assertion"
	case RuleTokenAssertion:
		return "token-assertion"
	case RuleBackgroundContext:
		return "background-context"
	case RuleBooleanIdentity:
		return "boolean-identity"
	case RuleUnpinnedRendering:
		return "unpinned-rendering"
	default:
		return "Rule(" + strconv.Itoa(int(r)) + ")"
	}
}

const testSuffix = "_test.go"

const analyzerDoc = `checks that specs assert which error they got, and which member, under the context they are handed

An assertion that some error occurred passes on the wrong failure, so in the
specs of an API package it is reported, and the spec asserts the error's type
and fields instead. An error's text and an enum's tokens are each asserted once,
in the test file pinning the rendering of the source that declares them;
everywhere else a positive assertion on either is reported. Opted in, an error
type whose rendering no such file pins is reported too. What errors.Is
or errors.As reported is asserted through the assertion that reports the error,
not as a boolean. A test takes the context its framework hands it rather than
context.Background.`

// NewAnalyzer builds the analyzer that judges specs under config.
func NewAnalyzer(config Config) *analysis.Analyzer {
	c := &checker{config: config}

	return &analysis.Analyzer{
		Name:      "spec",
		Doc:       analyzerDoc,
		Run:       c.run,
		FactTypes: []analysis.Fact{(*enumFact)(nil), (*renderingFact)(nil), (*pinsFact)(nil)},
	}
}

type checker struct {
	config Config
}

func (c *checker) run(pass *analysis.Pass) (any, error) {
	exportEnums(pass)
	api, err := c.config.identifies(pass.Pkg.Path(), pass.Pkg.Name())
	if err != nil {
		return nil, err
	}

	var p *passChecker
	var dir *directory
	tests := map[string]*ast.File{}
	for _, file := range pass.Files {
		// The name on disk, past any //line directive, is what a rendering pattern
		// and the sibling sources are matched against.
		path := pass.Fset.PositionFor(file.Package, false).Filename
		if !strings.HasSuffix(path, testSuffix) {
			continue
		}
		tests[path] = file
		// Every file of a package sits in one directory, so the first is as good
		// as any for naming it.
		if p == nil {
			dir = &directory{path: filepath.Dir(path)}
			p = newPassChecker(pass, moduleRoot(dir.path))
			p.disabled = c.config.Disabled
		}
		sources, err := c.renderedSources(filepath.Base(path), dir)
		if err != nil {
			return nil, err
		}
		pinsText, err := pinsText(sources)
		if err != nil {
			return nil, err
		}
		f := &fileChecker{passChecker: p, path: path, identify: api, pinsText: pinsText, sources: sources}
		f.check(file)
		p.backgroundContexts(file)
	}
	if slices.Contains(c.config.Disabled, RuleUnpinnedRendering) {
		return nil, nil
	}
	var pins map[string]map[typeKey]bool
	if p != nil {
		pins = p.pins
	}
	return nil, c.unpinned(pass, dir, tests, pins)
}

// passChecker is what the test files of one pass share: its type information,
// the writes to its variables, and what has been read of them.
type passChecker struct {
	pass *analysis.Pass

	// disabled are the rules that report nothing.
	disabled []Rule

	// root is the module root of the pass's test files, which an enum must
	// share to be judged.
	root string

	variables  map[*types.Var]*variable
	causes     map[*types.Var]bool
	texts      map[*types.Var]textResult
	renderings map[*types.Var][]enumRef
	identities map[*types.Var]identityResult

	// pins are the error types each test file, by path, pins the rendering of.
	pins map[string]map[typeKey]bool

	context     *types.Interface
	contextRead bool
}

func newPassChecker(pass *analysis.Pass, root string) *passChecker {
	return &passChecker{
		pass:       pass,
		root:       root,
		variables:  collectVariables(pass.Files, pass.TypesInfo),
		causes:     map[*types.Var]bool{},
		texts:      map[*types.Var]textResult{},
		renderings: map[*types.Var][]enumRef{},
		identities: map[*types.Var]identityResult{},
		pins:       map[string]map[typeKey]bool{},
	}
}

// fileChecker judges the assertions of one test file.
type fileChecker struct {
	*passChecker

	// path is the file's name on disk.
	path string

	// identify reports whether the file belongs to an API package's specs,
	// which is where unidentified-error is judged.
	identify bool

	// pinsText reports whether the file pins an error's text, which is where
	// text-assertion is not judged.
	pinsText bool

	// sources are the paths of the sources a rendering pattern names the file
	// for, whose enums' tokens it pins.
	sources []string
}

func (f *fileChecker) check(file *ast.File) {
	ast.PreorderStack(file, nil, func(n ast.Node, stack []ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			f.gomegaMatcher(call, stack)
			f.gomegaAssertion(call)
			f.gomegaPin(call)
			f.testify(call)
		}
		return true
	})
}

func (f *fileChecker) unidentified(pos token.Pos, message string) {
	if f.identify {
		f.report(pos, RuleUnidentifiedError, message)
	}
}

// Messages a text-assertion finding carries.
const (
	textMessage = "asserts an error's text, which only the spec of the source declaring its Error method pins; " +
		"assert the error's type and fields instead"
	builtMessage = "matches an error built in the spec, which Gomega compares by its text; " +
		"match the error's identity or type instead"
)

// textAsserted reports a positive assertion on an error's text. subjects
// are the errors whose text is asserted, where they are known: when every one
// is a cause, the assertion is on a rendering no type of its own pins.
func (f *fileChecker) textAsserted(pos token.Pos, message string, subjects ...ast.Expr) {
	if f.pinsText || f.allCauses(subjects) {
		return
	}
	f.report(pos, RuleTextAssertion, message)
}

// allCauses reports whether every subject is a cause. An unknown subject
// is nil, and no cause.
func (f *fileChecker) allCauses(subjects []ast.Expr) bool {
	if len(subjects) == 0 {
		return false
	}
	for _, subject := range subjects {
		if subject == nil || !f.isCause(subject) {
			return false
		}
	}
	return true
}

// tokenAsserted reports a positive assertion on the rendering of enums,
// unless the file pins the tokens of every one.
func (f *fileChecker) tokenAsserted(pos token.Pos, enums []enumRef) {
	for _, e := range enums {
		if !f.pinsTokens(e) {
			f.report(pos, RuleTokenAssertion, fmt.Sprintf("asserts the rendered token of %s, "+
				"which only the spec of the file declaring it or its String pins; assert the member instead",
				e.display()))
			return
		}
	}
}

// pinsTokens reports whether the file pins an enum's tokens: a rendering
// pattern names it for the file declaring the enum's type or its String.
func (f *fileChecker) pinsTokens(e enumRef) bool {
	return slices.ContainsFunc(f.sources, func(source string) bool {
		return source == e.fact.TypeFile || source == e.fact.StringFile
	})
}

// report reports a finding of rule, unless the configuration disables it.
func (p *passChecker) report(pos token.Pos, rule Rule, message string) {
	if slices.Contains(p.disabled, rule) {
		return
	}
	p.pass.Report(diagnostic(pos, rule, message))
}

// diagnostic is a finding of rule, its message opening with the rule's token.
func diagnostic(pos token.Pos, rule Rule, message string) analysis.Diagnostic {
	return analysis.Diagnostic{Pos: pos, Category: rule.String(), Message: rule.String() + ": " + message}
}
