package zlines

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"os"
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

// RuleCommentWrap fires on a comment paragraph with a line outside the band
// the comment settings draw — past the limit, short of the minimum while
// the next word would still fit — or ending on a word that belongs
// with the line below.
const RuleCommentWrap Rule = "comment-wrap"

const analyzerDoc = `checks for line breaks a declaration should not have, and line breaks it should

A function signature spread over several lines that would fit within the
configured line length on one is reported, with a fix that joins it. A function
body written on the line its signature ends on is reported whatever the line
length, with a fix that puts it on lines of its own. A comment paragraph with
a line outside the band between the comment minimum and the comment length, or
with a line ending on a word that belongs with the next, is reported with a fix
that fills it.`

// NewAnalyzer builds the analyzer that enforces the line-break conventions
// under settings.
func NewAnalyzer(settings Settings) *analysis.Analyzer {
	c := &checker{
		lineLength:       settings.lineLength(),
		commentLength:    settings.commentLength(),
		commentMinLength: settings.commentMinLength(),
		signatureWrap:    settings.signatureWrap(),
		bodyCollapse:     settings.bodyCollapse(),
		commentWrap:      settings.commentWrap(),
		commentExempt:    settings.commentExempt(),
	}

	return &analysis.Analyzer{
		Name: Name,
		Doc:  analyzerDoc,
		Run:  c.run,
	}
}

type checker struct {
	lineLength       int
	commentLength    int
	commentMinLength int
	signatureWrap    bool
	bodyCollapse     bool
	commentWrap      bool
	commentExempt    []string
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

// checkComments reports every comment paragraph with a line the band does not
// admit, filling the whole paragraph to the limit as the fix.
func (c *checker) checkComments(pass *analysis.Pass, src []byte, file *token.File, syntax *ast.File, newline string) {
	for _, group := range syntax.Comments {
		for _, para := range paragraphs(src, file, group, c.commentExempt) {
			carried := utf8.RuneCountInString(para.indent) + len("//")
			first := carried + utf8.RuneCountInString(para.lead)
			rest := carried + utf8.RuneCountInString(para.hang)

			broke := para.banded(first, rest, c.commentMinLength, c.commentLength)
			if broke == held {
				continue
			}

			filled := fill(first, rest, c.commentLength, para.words())

			// A fill that changes the line count says so either way, since it can split
			// a paragraph as well as gather one; the same count means the words moved
			// but the shape did not, and the message names what put a line out
			// of the band instead.
			message := fmt.Sprintf("%s: comment wraps over %d lines; filled to %d columns it takes %d",
				RuleCommentWrap, len(para.lines), c.commentLength, len(filled))
			if len(filled) == len(para.lines) {
				message = c.bandMessage(broke)
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

// bandMessage says what put a line out of the band when the fill does not
// change the paragraph's line count.
func (c *checker) bandMessage(broke breach) string {
	switch broke {
	case overrun:
		return fmt.Sprintf("%s: comment runs past the %d-column limit", RuleCommentWrap, c.commentLength)
	case stranded:
		return fmt.Sprintf("%s: comment breaks after a word that belongs with the line below", RuleCommentWrap)
	default:
		return fmt.Sprintf("%s: comment is wrapped short of the %d-column minimum",
			RuleCommentWrap, c.commentMinLength)
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
