package zlines_test

import (
	"cmp"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/zcayou/go-tools/golangci/zlines"
)

// A fixture for one rule carries prose written for the reader rather than
// filled to the limit, so every suite but the comment-wrap one turns that rule
// off and keeps each fixture about the rule it is named for.
var quiet = zlines.Settings{CommentWrap: new(false)}

// bandedComment breaks inside the band a comment length of 30 derives: its
// first line stops at 24 columns, in the 20-to-30 band, though the next word
// would still fit on it.
const bandedComment = "package p\n" +
	"\n" +
	"// alpha beta gamma sums\n" +
	"// zeta eta theta\n" +
	"type banded struct{}\n"

var _ = Describe("Analyzer", func() {
	It("is named for the linter", func() {
		Expect(zlines.NewAnalyzer(zlines.Settings{}).Name).To(Equal(zlines.Name))
	})

	It("reports every wrapped signature that fits, and joins it", func() {
		analysistest.RunWithSuggestedFixes(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(quiet), "wrapped")
	})

	It("renders the joined signature the way gofmt would", func() {
		results := analysistest.Run(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(quiet), "wrapped")

		Expect(fixText(results, zlines.RuleSignatureWrap, "params")).To(
			Equal("func params(count int, name string) error"))
		Expect(fixText(results, zlines.RuleSignatureWrap, "oneFieldStruct")).To(
			Equal("func oneFieldStruct(s struct{ Count int }) error"))
	})

	It("leaves a wrap alone when joining would lose something or not last", func() {
		analysistest.Run(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(quiet), "justified")
	})

	It("holds signatures to the configured limit, which they may reach exactly", func() {
		settings := quiet
		settings.LineLength = new(34)

		analysistest.Run(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(settings), "narrow")
	})

	It("reports a body written on the signature line, and puts it on its own", func() {
		analysistest.RunWithSuggestedFixes(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(quiet), "collapsed")
	})

	It("renders the expanded body the way gofmt would, indentation and all", func() {
		results := analysistest.Run(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(quiet), "collapsed")

		Expect(fixText(results, zlines.RuleBodyCollapse, "Demand")).To(Equal("{\n\treturn r.demand\n}"))
		Expect(fixText(results, zlines.RuleBodyCollapse, "several")).To(Equal("{\n\tfirst()\n\tsecond()\n}"))
	})

	It("fills every comment paragraph, and never across a blank //-line", func() {
		analysistest.RunWithSuggestedFixes(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(zlines.Settings{}), "comments")
	})

	It("carries the indent onto every line it fills, and says what it did", func() {
		results := analysistest.Run(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(zlines.Settings{}), "comments")

		Expect(messages(results)).To(ContainElement(string(zlines.RuleCommentWrap) +
			": comment wraps over 3 lines; filled to 80 columns it takes 1"))
		Expect(edits(results)).To(ContainElement("// this is a short comment blah"))
		Expect(edits(results)).To(ContainElement(
			"// a field comment written over three short lines whose words still need two" +
				"\n\t// lines once they are filled to the limit"))
	})

	It("tolerates breaks inside the band, and polices its edges", func() {
		analysistest.RunWithSuggestedFixes(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(zlines.Settings{}), "banded")
	})

	It("derives the minimum from the comment length, ten columns under it", func() {
		Expect(analyze(zlines.NewAnalyzer(zlines.Settings{CommentLength: new(30)}), bandedComment).messages()).
			To(BeEmpty())
	})

	It("restores the exact fill when the minimum meets the length", func() {
		analyzer := zlines.NewAnalyzer(zlines.Settings{CommentLength: new(30), CommentMinLength: new(30)})

		Expect(analyze(analyzer, bandedComment).messages()).To(ConsistOf(string(zlines.RuleCommentWrap) +
			": comment is wrapped short of the 30-column minimum"))
	})

	It("says a comment splits rather than gathers when that is what filling it does", func() {
		const src = "package p\n" +
			"\n" +
			"// one line of prose wider than the limit it is held to\n" +
			"type split struct{}\n"

		Expect(analyze(zlines.NewAnalyzer(zlines.Settings{CommentLength: new(30)}), src).messages()).To(
			ConsistOf(string(zlines.RuleCommentWrap) +
				": comment wraps over 1 lines; filled to 30 columns it takes 3"))
	})

	It("fills a list item inside the marker it was written with", func() {
		results := analysistest.Run(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(zlines.Settings{}), "comments")

		Expect(edits(results)).To(ContainElement(
			"//   - an item whose text is wrapped well short of the limit"))
	})

	It("reads a list where gofmt reads one, and a marker at the margin as prose", func() {
		analysistest.RunWithSuggestedFixes(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(zlines.Settings{}), "lists")
	})

	It("does not leave a line ending on a word that belongs with the next", func() {
		results := analysistest.Run(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(zlines.Settings{}), "comments")

		// Greedy alone fits "the" onto the first line at 77 columns and strands
		// it there, so this pins the shorter line rather than the fuller one.
		Expect(edits(results)).To(ContainElement(
			"// Centralizing the checks keeps them consistent across tiers and absorbs" +
				"\n// the variation in the output shapes."))
	})

	It("leaves a comment addressed to a tool exactly as it was written", func() {
		analysistest.Run(GinkgoT(), analysistest.TestData(), zlines.NewAnalyzer(zlines.Settings{
			CommentExempt: []string{"+kubebuilder"},
		}), "exemptmarkers")
	})

	It("leaves a build constraint and a generated-file marker out of the prose below", func() {
		analysistest.Run(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(zlines.Settings{}), "directives")
	})

	It("finds a line by scanning the source, whatever a //line directive numbers it", func() {
		analysistest.Run(GinkgoT(), analysistest.TestData(),
			zlines.NewAnalyzer(zlines.Settings{}), "linedirective")
	})

	It("leaves every rule alone when it is turned off", func() {
		analysistest.Run(GinkgoT(), analysistest.TestData(), zlines.NewAnalyzer(zlines.Settings{
			SignatureWrap: new(false),
			BodyCollapse:  new(false),
			CommentWrap:   new(false),
		}), "unchecked")
	})

	It("writes a fill gofmt does not take back", func() {
		// A dash at the margin is prose to gofmt. Filling it as a list item would
		// hang its continuation under a marker gofmt does not see there, and gofmt
		// would move the marker to meet it.
		const src = "package p\n" +
			"\n" +
			"// a paragraph whose middle line opens with a dash\n" +
			"// - which is prose, since a marker at the margin is not indented\n" +
			"// and which therefore joins the lines around it\n" +
			"type unindented struct{}\n"

		fixed := analyze(zlines.NewAnalyzer(zlines.Settings{}), src).fixed()

		Expect(fixed).NotTo(Equal(src))
		Expect(fixed).To(Equal(gofmt(fixed)))
	})

	It("breaks the lines it writes the way the file breaks its own", func() {
		// The testdata tree is pinned to LF by .gitattributes, so a file written
		// with CRLF has to be built here to be put to the rules at all.
		const src = "package p\r\n" +
			"\r\n" +
			"// a comment\r\n" +
			"// wrapped short\r\n" +
			"func flat() { first(); second() }\r\n" +
			"\r\n" +
			"func first() {}\r\n" +
			"\r\n" +
			"func second() {}\r\n"

		// The minimum is pinned to the length because the comment's first line sits
		// inside the derived band, and this spec is about line terminators.
		fixed := analyze(zlines.NewAnalyzer(zlines.Settings{
			CommentLength:    new(20),
			CommentMinLength: new(20),
		}), src).fixed()

		Expect(fixed).To(Equal("package p\r\n" +
			"\r\n" +
			"// a comment wrapped\r\n" +
			"// short\r\n" +
			"func flat() {\r\n" +
			"\tfirst()\r\n" +
			"\tsecond()\r\n" +
			"}\r\n" +
			"\r\n" +
			"func first() {}\r\n" +
			"\r\n" +
			"func second() {}\r\n"))
	})
})

// analyzed is what running an analyzer over source held in memory produced.
// It is how source the testdata tree cannot hold is put to the rules.
type analyzed struct {
	fset  *token.FileSet
	src   string
	diags []analysis.Diagnostic
}

// analyze runs analyzer over src as a file of its own.
func analyze(analyzer *analysis.Analyzer, src string) analyzed {
	GinkgoHelper()

	fset := token.NewFileSet()
	syntax, err := parser.ParseFile(fset, "memory.go", src, parser.ParseComments)
	Expect(err).NotTo(HaveOccurred())

	out := analyzed{fset: fset, src: src}
	_, err = analyzer.Run(&analysis.Pass{
		Fset:     fset,
		Files:    []*ast.File{syntax},
		ReadFile: func(string) ([]byte, error) { return []byte(src), nil },
		Report:   func(diagnostic analysis.Diagnostic) { out.diags = append(out.diags, diagnostic) },
	})
	Expect(err).NotTo(HaveOccurred())

	return out
}

// messages returns what every diagnostic said.
func (a analyzed) messages() []string {
	out := make([]string, 0, len(a.diags))
	for _, diagnostic := range a.diags {
		out = append(out, diagnostic.Message)
	}

	return out
}

// fixed returns the source with every offered fix applied, which is what --fix
// would leave.
func (a analyzed) fixed() string {
	GinkgoHelper()

	var edits []analysis.TextEdit
	for _, diagnostic := range a.diags {
		for _, fix := range diagnostic.SuggestedFixes {
			edits = append(edits, fix.TextEdits...)
		}
	}
	slices.SortFunc(edits, func(one, two analysis.TextEdit) int { return cmp.Compare(one.Pos, two.Pos) })

	var out strings.Builder
	written := 0
	for _, edit := range edits {
		start, end := a.fset.Position(edit.Pos).Offset, a.fset.Position(edit.End).Offset
		Expect(start).To(BeNumerically(">=", written), "fixes overlap")

		out.WriteString(a.src[written:start])
		out.Write(edit.NewText)
		written = end
	}
	out.WriteString(a.src[written:])

	return out.String()
}

// gofmt returns src as the formatter would write it.
func gofmt(src string) string {
	GinkgoHelper()

	formatted, err := format.Source([]byte(src))
	Expect(err).NotTo(HaveOccurred())

	return string(formatted)
}

// fixText returns the text the fix rule offered for the declaration named name
// would write. It is what neither the golden files nor golangci-lint can pin
// down: both run the result through gofmt before anyone compares it, so a fix
// that laid out its own text badly would pass either one.
func fixText(results []*analysistest.Result, rule zlines.Rule, name string) string {
	GinkgoHelper()

	for _, result := range results {
		for _, diagnostic := range result.Diagnostics {
			if !strings.HasPrefix(diagnostic.Message, string(rule)+": "+name+": ") {
				continue
			}

			Expect(diagnostic.SuggestedFixes).To(HaveLen(1))
			Expect(diagnostic.SuggestedFixes[0].TextEdits).To(HaveLen(1))

			return string(diagnostic.SuggestedFixes[0].TextEdits[0].NewText)
		}
	}

	Fail("no " + string(rule) + " diagnostic for " + name)

	return ""
}

// messages returns every diagnostic message the run produced.
func messages(results []*analysistest.Result) []string {
	var out []string
	for _, result := range results {
		for _, diagnostic := range result.Diagnostics {
			out = append(out, diagnostic.Message)
		}
	}

	return out
}

// edits returns the text every offered fix would write, which the golden files
// see only after gofmt has been over it.
func edits(results []*analysistest.Result) []string {
	var out []string
	for _, result := range results {
		for _, diagnostic := range result.Diagnostics {
			for _, fix := range diagnostic.SuggestedFixes {
				for _, edit := range fix.TextEdits {
					out = append(out, string(edit.NewText))
				}
			}
		}
	}

	return out
}
