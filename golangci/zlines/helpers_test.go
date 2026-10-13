package zlines_test

import (
	"cmp"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis"
)

// loneComment is a paragraph of one line reaching 82 columns, two past
// the default limit and inside the default overrun.
const loneComment = "package p\n" +
	"\n" +
	"// this single line of prose runs a couple of columns past the eighty column limit\n" +
	"type lone struct{}\n"

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
