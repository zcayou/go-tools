package zlines

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"slices"
	"strings"
)

// bodyPrefix gives a body a signature of its own to be rendered under, so
// that gofmt lays it out as it lays out any function. Nothing here is type
// checked, so the signature need not agree with what the body does.
const bodyPrefix = "package p\nfunc _() "

// collapsed reports whether body is written on the line its opening brace sits
// on. A body declaring no statements is not collapsed onto anything: gofmt
// writes an empty body as {}, and a body holding only a comment or a bare
// semicolon has nothing in it that belongs on a line of its own.
func collapsed(file *token.File, body *ast.BlockStmt) bool {
	if body == nil || file.Line(body.Lbrace) != file.Line(body.Rbrace) {
		return false
	}

	return slices.ContainsFunc(body.List, func(stmt ast.Stmt) bool {
		_, blank := stmt.(*ast.EmptyStmt)

		return !blank
	})
}

// expand renders decl's collapsed body the way gofmt would write it over lines
// of its own, braces included, ready to replace the body in the source,
// with newline for the line breaks it writes. It reports false when
// that rendering fails or comes back on one line after all, which the source
// this is handed cannot produce; the check is there because [format.Source]
// is a boundary.
func expand(src []byte, file *token.File, decl *ast.FuncDecl, newline string) (string, bool) {
	inner := bytes.TrimSpace(src[file.Offset(decl.Body.Lbrace)+1 : file.Offset(decl.Body.Rbrace)])

	formatted, err := format.Source(slices.Concat([]byte(bodyPrefix+"{\n"), inner, []byte("\n}\n")))
	if err != nil {
		return "", false
	}

	// The prefix carries no braces of its own, so the first one opens the body.
	open := bytes.IndexByte(formatted, '{')
	if open < 0 {
		return "", false
	}

	text := strings.TrimRight(string(formatted[open:]), "\n")
	if !strings.Contains(text, "\n") {
		return "", false
	}

	// gofmt indents the rendering for a declaration at the margin, which is where
	// it puts every one. A file it has not been over can have the declaration
	// elsewhere, and each line has to carry that indent to land under it.
	indent := lineIndent(src, file, decl.Pos())

	return strings.ReplaceAll(text, "\n", newline+indent), true
}

// lineIndent returns the whitespace the line holding pos opens with.
func lineIndent(src []byte, file *token.File, pos token.Pos) string {
	return indentOf(lineUpTo(src, file, pos))
}

// lineUpTo returns the bytes between the start of pos's physical line and pos
// itself. A line is found by scanning the source rather than by number, because
// a //line directive moves the numbers a position reports away from the lines
// the source actually has.
func lineUpTo(src []byte, file *token.File, pos token.Pos) []byte {
	offset := file.Offset(pos)

	return src[bytes.LastIndexByte(src[:offset], '\n')+1 : offset]
}

// indentOf returns the whitespace prefix opens with.
func indentOf(prefix []byte) string {
	return string(prefix[:len(prefix)-len(bytes.TrimLeft(prefix, " \t"))])
}
