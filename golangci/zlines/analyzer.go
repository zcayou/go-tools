package zlines

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"slices"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
)

// Rule names the check a diagnostic came from.
//
// golangci-lint discards [analysis.Diagnostic.Category], so the rule is also
// the first word of every message; that is what makes it addressable
// by a linters.exclusions text rule.
type Rule string

// RuleSignatureWrap fires on a function signature spread over several lines
// that would fit inside the line-length limit on one, which makes its line
// breaks noise.
const RuleSignatureWrap Rule = "signature-wrap"

// RuleBodyCollapse fires on a function body written onto the line its signature
// ends on, where a reader going down a file's declarations does not expect
// to find code.
const RuleBodyCollapse Rule = "body-collapse"

// RuleCommentWrap fires on a comment paragraph whose line breaks are not
// the ones filling it to the comment limit would give it.
const RuleCommentWrap Rule = "comment-wrap"

const analyzerDoc = `checks for line breaks a declaration should not have, and line breaks it should

A function signature spread over several lines that would fit within the
configured line length on one is reported, with a fix that joins it. A function
body written on the line its signature ends on is reported whatever the line
length, with a fix that puts it on lines of its own. A comment paragraph whose
lines are not filled to the comment length is reported, with a fix that fills
them.`

// NewAnalyzer builds the analyzer that enforces the line-break conventions
// under settings.
func NewAnalyzer(settings Settings) *analysis.Analyzer {
	c := &checker{
		lineLength:    settings.lineLength(),
		commentLength: settings.commentLength(),
		signatureWrap: settings.signatureWrap(),
		bodyCollapse:  settings.bodyCollapse(),
		commentWrap:   settings.commentWrap(),
		commentExempt: settings.commentExempt(),
	}

	return &analysis.Analyzer{
		Name: Name,
		Doc:  analyzerDoc,
		Run:  c.run,
	}
}

type checker struct {
	lineLength    int
	commentLength int
	signatureWrap bool
	bodyCollapse  bool
	commentWrap   bool
	commentExempt []string
}

func (c *checker) run(pass *analysis.Pass) (any, error) {
	for _, syntax := range pass.Files {
		file := pass.Fset.File(syntax.FileStart)
		if file == nil {
			continue
		}

		src, err := read(pass, file.Name())
		if err != nil {
			return nil, err
		}
		// Positions index the source as it was parsed, so a file that has been
		// written to since would take its edits at the wrong offsets.
		if len(src) != file.Size() {
			continue
		}

		newline := terminator(src)

		for _, decl := range syntax.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if c.signatureWrap {
				c.checkSignature(pass, src, file, fn)
			}
			if c.bodyCollapse {
				c.checkBody(pass, src, file, fn, newline)
			}
		}

		if c.commentWrap {
			c.checkComments(pass, src, file, syntax, newline)
		}
	}

	return nil, nil
}

func (c *checker) checkSignature(pass *analysis.Pass, src []byte, file *token.File, decl *ast.FuncDecl) {
	sig, ok := join(src, file, decl)
	if !ok || sig.width > c.lineLength {
		return
	}

	pass.Report(analysis.Diagnostic{
		Pos:      decl.Pos(),
		End:      decl.Type.End(),
		Category: string(RuleSignatureWrap),
		Message: fmt.Sprintf("%s: %s: signature is wrapped but fits on one %d-column line (limit %d)",
			RuleSignatureWrap, decl.Name.Name, sig.width, c.lineLength),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "join the signature onto one line",
			TextEdits: []analysis.TextEdit{{
				Pos:     decl.Pos(),
				End:     decl.Type.End(),
				NewText: []byte(sig.text),
			}},
		}},
	})
}

// checkBody reports a body written on the line its signature ends on. Unlike
// a wrap that stands, the diagnostic does not wait on the rendering: no line
// length makes a folded body the right way to write the declaration.
func (c *checker) checkBody(pass *analysis.Pass, src []byte, file *token.File, decl *ast.FuncDecl, newline string) {
	if !collapsed(file, decl.Body) {
		return
	}

	diagnostic := analysis.Diagnostic{
		Pos:      decl.Body.Lbrace,
		End:      decl.Body.End(),
		Category: string(RuleBodyCollapse),
		Message: fmt.Sprintf("%s: %s: body is written on the signature line rather than lines of its own",
			RuleBodyCollapse, decl.Name.Name),
	}

	if text, ok := expand(src, file, decl, newline); ok {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "put the body on lines of its own",
			TextEdits: []analysis.TextEdit{{
				Pos:     decl.Body.Lbrace,
				End:     decl.Body.End(),
				NewText: []byte(text),
			}},
		}}
	}

	pass.Report(diagnostic)
}

// checkComments reports every comment paragraph whose line breaks are not
// the ones filling it would give it.
func (c *checker) checkComments(pass *analysis.Pass, src []byte, file *token.File, syntax *ast.File, newline string) {
	for _, group := range syntax.Comments {
		for _, para := range paragraphs(src, file, group, c.commentExempt) {
			carried := utf8.RuneCountInString(para.indent) + len("//")

			filled := fill(carried+utf8.RuneCountInString(para.lead),
				carried+utf8.RuneCountInString(para.hang), c.commentLength, para.words())
			if slices.Equal(filled, para.lines) {
				continue
			}

			// A fill that changes the line count says so either way, since it can split
			// a paragraph as well as gather one; the same count means the words moved
			// but the shape did not.
			message := fmt.Sprintf("%s: comment wraps over %d lines; filled to %d columns it takes %d",
				RuleCommentWrap, len(para.lines), c.commentLength, len(filled))
			if len(filled) == len(para.lines) {
				message = fmt.Sprintf("%s: comment is wrapped short of the %d-column limit",
					RuleCommentWrap, c.commentLength)
			}

			pass.Report(analysis.Diagnostic{
				Pos:      para.pos,
				End:      para.end,
				Category: string(RuleCommentWrap),
				Message:  message,
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "fill the comment to the limit",
					TextEdits: []analysis.TextEdit{{
						Pos:     para.pos,
						End:     para.end,
						NewText: []byte(para.text(filled, newline)),
					}},
				}},
			})
		}
	}
}

// terminator returns the line ending src is written with, which a fix writing
// lines of its own has to reproduce rather than leave the file with two kinds.
func terminator(src []byte) string {
	if bytes.Contains(src, []byte("\r\n")) {
		return "\r\n"
	}

	return "\n"
}

// read returns the contents of filename, through the driver's reader when
// it supplies one so that a virtualized file tree is honored.
func read(pass *analysis.Pass, filename string) ([]byte, error) {
	readFile := os.ReadFile
	if pass.ReadFile != nil {
		readFile = pass.ReadFile
	}

	src, err := readFile(filename)
	if err != nil {
		return nil, fmt.Errorf("%s: reading %s: %w", Name, filename, err)
	}

	return src, nil
}
