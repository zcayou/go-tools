package zlines

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/scanner"
	"go/token"
	"slices"
	"strings"
	"unicode/utf8"
)

// joined is a wrapped function signature as it would read on one line.
type joined struct {
	text  string // the signature, rendered the way gofmt renders it
	width int    // columns the line holding it would occupy
}

// join works out how decl's signature reads with its line breaks taken out.
// It reports false when there is nothing to join, and when joining would not
// be safe; the package documentation lists the wraps that stand.
//
// The width covers the whole line the fix would produce, not just
// the signature: whatever precedes the func keyword on its line, and whatever
// follows the signature on the last one — the opening brace, and a trailing
// comment if there is one. A collapsed body is not part of that line, since
// the fix for it takes the body off.
func join(src []byte, file *token.File, decl *ast.FuncDecl) (joined, bool) {
	start, end := file.Offset(decl.Pos()), file.Offset(decl.Type.End())
	if file.Line(decl.Pos()) == file.Line(decl.Type.End()) {
		return joined{}, false
	}

	flat, ok := flatten(src[start:end])
	if !ok {
		return joined{}, false
	}

	text, ok := gofmtLine(flat)
	if !ok {
		return joined{}, false
	}

	rest := restOfLine(src, end)
	// A collapsed body draws a diagnostic of its own, and its fix takes the body
	// off this line. What the two fixes leave is the signature and the brace, so
	// that is what the width has to be of.
	if collapsed(file, decl.Body) {
		rest = rest[:min(file.Offset(decl.Body.Lbrace)+1-end, len(rest))]
	}

	return joined{
		text: text,
		width: utf8.RuneCount(lineUpTo(src, file, decl.Pos())) +
			utf8.RuneCountInString(text) +
			utf8.RuneCount(rest),
	}, true
}

// flatten takes the line breaks out of src, a function signature, leaving
// syntax that parses the same. It reports false when a comment stands
// in the way; the package documentation lists which comments those are.
func flatten(src []byte) ([]byte, bool) {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))

	var s scanner.Scanner
	var failed bool
	s.Init(file, src, func(token.Position, string) { failed = true }, scanner.ScanComments)

	var out []byte
	prevEnd := 0
	prevWord := false
	commentOpenedLine := false

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		text, size := source(tok, lit)
		offset := file.Offset(pos)
		gap := src[prevEnd:offset]

		// A block comment with a line to itself documents what follows it. On one
		// line gofmt sits it after the parameter above instead, which reads
		// as documenting the wrong one, so the wrap stands.
		if commentOpenedLine && bytes.ContainsRune(gap, '\n') {
			return nil, false
		}
		commentOpenedLine = false

		if tok == token.COMMENT {
			// A line comment would swallow the rest of the joined line, and a comment
			// spanning lines cannot sit on one.
			if strings.HasPrefix(lit, "//") || strings.Contains(lit, "\n") {
				return nil, false
			}
			commentOpenedLine = bytes.ContainsRune(gap, '\n')
		}

		switch {
		case len(out) == 0:
		case closes(tok):
			// A separator before a closing bracket is what lets the bracket sit
			// on a line of its own; on one line it is not written.
			out = bytes.TrimRight(out, ",;")
		case !bytes.ContainsRune(gap, '\n'):
			out = append(out, gap...)
		case prevWord && isWord(tok):
			out = append(out, ' ')
		}

		out = append(out, text...)
		prevEnd = offset + size
		prevWord = isWord(tok)
	}

	if failed {
		return nil, false
	}

	// The scanner closes the fragment with a semicolon standing for the line break
	// that follows the signature in the file.
	return bytes.TrimRight(out, ";"), true
}

// source returns the text to write for tok and the number of bytes it occupies
// in the source. A semicolon the scanner inserted for a line break is written
// out, because inside an inline struct or interface type it separates fields
// rather than statements.
func source(tok token.Token, lit string) (text string, size int) {
	if tok == token.SEMICOLON {
		return ";", 1 // lit is "\n" when the scanner inserted it
	}
	if lit != "" {
		return lit, len(lit)
	}

	return tok.String(), len(tok.String())
}

func closes(tok token.Token) bool {
	return tok == token.RPAREN || tok == token.RBRACK || tok == token.RBRACE
}

// isWord reports whether tok needs whitespace to stay a token of its own.
// [token.Token.IsLiteral] covers identifiers as well as literals.
func isWord(tok token.Token) bool {
	return tok.IsLiteral() || tok.IsKeyword()
}

// gofmtLine renders sig as gofmt would, and reports false unless that takes
// a single line. gofmt spreads some types over several lines however they were
// written — a struct type holding more than one field, for one — and a fix
// gofmt would undo on the next save is no fix at all.
func gofmtLine(sig []byte) (string, bool) {
	// A signature with no body is a declaration in its own right, so this parses
	// as a file without needing a body bolted on.
	formatted, err := format.Source(slices.Concat([]byte("package p\n"), sig, []byte("\n")))
	if err != nil {
		return "", false
	}

	var only string
	for line := range strings.Lines(string(formatted)) {
		line = strings.TrimRight(line, "\n")
		if line == "" || strings.HasPrefix(line, "package ") {
			continue
		}
		if only != "" {
			return "", false
		}
		only = line
	}

	return only, only != ""
}

// restOfLine returns what follows offset on its line, which a joined signature
// would share the line with.
func restOfLine(src []byte, offset int) []byte {
	rest := src[offset:]
	if i := bytes.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[:i]
	}

	return bytes.TrimRight(rest, " \t\r")
}
