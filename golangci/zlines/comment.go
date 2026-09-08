package zlines

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// directive matches a comment line addressed to a tool rather than a reader: a Go directive such
	// as //go:build or //nolint:zlines, the legacy build constraint go vet holds to the //go:build
	// line beside it, and the marker that makes a file generated. The last two read as prose, and
	// joining either to the sentence below it takes the line out of the form its tool matches on.
	directive = regexp.MustCompile(`^//[a-z0-9]+:[^ \t]|^// \+build( |$)|^// Code generated .* DO NOT EDIT\.$`)

	// listMarker matches the bullet or number opening a doc-comment list item,
	// capturing everything up to the item's text so a fill can reproduce it.
	// go/doc reads a list only where the line is still indented once the prose
	// space is off, so a marker at the margin is a sentence that starts
	// with a dash, and treating it as an item would hang its continuation lines
	// under a list gofmt does not see there.
	listMarker = regexp.MustCompile(`^ [ \t]+(?:[-*+•]|\d+[.)])[ \t]`)

	// linkDef matches a doc-comment link definition, whose line is its whole
	// meaning.
	linkDef = regexp.MustCompile(`^\[[^\]]+\]:\s`)
)

// dangling are the words a line is not left ending on. Joining to what follows
// is the whole of what they do, so a break after one hands the reader
// an article or a preposition with nothing yet attached to it.
var dangling = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "to": true, "in": true,
	"on": true, "and": true, "or": true, "for": true, "with": true,
	"that": true, "is": true, "as": true, "by": true, "at": true,
	"it": true, "be": true, "are": true,
}

// prosePrefix is what follows the // of an ordinary sentence line.
const prosePrefix = " "

// kind says what a comment line is, which decides whether it can be filled
// and what it is filled inside of.
type kind int

const (
	// ends anything that ends the run it meets rather than joining it.
	ends kind = iota
	// sentence is an ordinary prose line.
	sentence
	// opener is the first line of a list item, which carries the marker.
	opener
	// hanging is a line indented under the line above it, which continues a list
	// item and is a code block anywhere else.
	hanging
)

// paragraph is a run of lines the rule fills as one: a prose paragraph,
// or a single list item together with the lines hanging under it. The line
// breaks inside it carry nothing, which is what makes it fillable; the break
// that carries something is the blank //-line, and that ends the run.
type paragraph struct {
	pos    token.Pos // start of the first comment in the run
	end    token.Pos // end of the last
	indent string    // whitespace before the // in the file
	lead   string    // what follows the // on the first line
	hang   string    // what follows the // on the rest
	lines  []string  // text of each line, past its prefix
}

// words returns the paragraph's text with its line breaks dropped.
func (p paragraph) words() []string {
	return strings.Fields(strings.Join(p.lines, " "))
}

// item reports whether the paragraph is a list item rather than prose.
func (p paragraph) item() bool {
	return p.lead != prosePrefix
}

// text renders lines as the comment the fix would write, breaking them
// with newline. Every line but the first carries the file indent, which
// the first already has in the source, and the first carries the marker while
// the rest hang under it.
func (p paragraph) text(lines []string, newline string) string {
	var out strings.Builder

	for i, line := range lines {
		if i > 0 {
			out.WriteString(newline)
			out.WriteString(p.indent)
		}

		out.WriteString("//")
		if i == 0 {
			out.WriteString(p.lead)
		} else {
			out.WriteString(p.hang)
		}
		out.WriteString(line)
	}

	return out.String()
}

// exempt reports whether the paragraph carries a token addressed to a tool.
// A directive is read from the start of a line, so a word the fill could put
// there is reason enough to leave the whole run as it was written.
func (p paragraph) exempt(prefixes []string) bool {
	for _, line := range p.lines {
		for word := range strings.FieldsSeq(line) {
			if exempted(word, prefixes) {
				return true
			}
		}
	}

	return false
}

// exempted reports whether text opens with one of prefixes. The leading slashes
// and spaces come off first, the way golangci-lint takes them off before looking
// for a nolint directive, so a quoted //nolint reads as the directive it would
// become.
func exempted(text string, prefixes []string) bool {
	text = strings.TrimLeft(text, "/ ")

	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}

	return false
}

// paragraphs splits group into the runs the rule may fill.
func paragraphs(src []byte, file *token.File, group *ast.CommentGroup, prefixes []string) []paragraph {
	var out []paragraph
	var run *paragraph

	flush := func() {
		if run != nil && !run.exempt(prefixes) {
			out = append(out, *run)
		}
		run = nil
	}

	for _, comment := range group.List {
		indent, ok := indentBefore(src, file, comment.Pos())
		if !ok {
			flush()
			continue
		}

		what, prefix, text := classify(comment, prefixes)

		switch what {
		case sentence:
			// A sentence ends a list item: it is outdented past the marker, so it is no
			// longer under it.
			if run != nil && (run.item() || run.indent != indent) {
				flush()
			}
			if run == nil {
				run = &paragraph{
					pos: comment.Pos(), indent: indent,
					lead: prosePrefix, hang: prosePrefix,
				}
			}

		case opener:
			// Each item fills on its own, inside the marker it was written with.
			flush()
			run = &paragraph{
				pos: comment.Pos(), indent: indent,
				lead: prefix,
				hang: strings.Repeat(" ", utf8.RuneCountInString(prefix)),
			}

		case hanging:
			// Indented under an open item this continues it; anywhere else it is a code
			// block, which is written the way it is on purpose.
			if run == nil || !run.item() || run.indent != indent {
				flush()
				continue
			}

		default:
			flush()
			continue
		}

		run.end = comment.End()
		run.lines = append(run.lines, text)
	}
	flush()

	return out
}

// classify says what a comment line is, returning the marker an opener carries
// — the one thing a fill has to reproduce rather than derive — and the text
// past it.
func classify(comment *ast.Comment, prefixes []string) (kind, string, string) {
	if !strings.HasPrefix(comment.Text, "//") || directive.MatchString(comment.Text) {
		return ends, "", ""
	}

	// A line addressed to a tool is written the way that tool reads it.
	if exempted(comment.Text, prefixes) {
		return ends, "", ""
	}

	// gofmt writes prose as "// " and gives every other shape a marker of its own,
	// so the space is what tells a sentence from commented-out code.
	body := comment.Text[len("//"):]
	if !strings.HasPrefix(body, prosePrefix) {
		return ends, "", ""
	}

	body = strings.TrimRight(body, " \t")

	if prefix := listMarker.FindString(body); prefix != "" {
		return opener, prefix, body[len(prefix):]
	}

	// Past the one space every line carries, what is left is an indent,
	// and an indent means the line hangs under the one above it.
	if prefix := body[:len(body)-len(strings.TrimLeft(body, " \t"))]; len(prefix) > len(prosePrefix) {
		return hanging, "", body[len(prefix):]
	}

	text := strings.TrimPrefix(body, prosePrefix)
	switch {
	// A blank //-line reaches this only in a file gofmt has not been over, which
	// can leave the space the bare // form has nothing after.
	case text == "":
		return ends, "", ""
	case strings.HasPrefix(text, "#"): // heading
		return ends, "", ""
	case linkDef.MatchString(text):
		return ends, "", ""
	// A line closing on a brace is code someone commented out, which filling would
	// run together into nonsense. A semicolon is not the same signal: gofmt never
	// ends a line with one, while prose does.
	case strings.HasSuffix(text, "{"), strings.HasSuffix(text, "}"):
		return ends, "", ""
	}

	return sentence, "", text
}

// fill lays words out greedily over lines no wider than limit, the first line
// carrying first columns before its text and the rest carrying rest. A word
// wider than the limit on its own takes a line of its own and overruns it,
// there being nowhere to break it.
func fill(first, rest, limit int, words []string) []string {
	var lines []string

	line, carried := "", first
	for _, word := range words {
		switch {
		case line == "":
			line = word

		case carried+utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) <= limit:
			line += " " + word

		default:
			kept, moved, ok := demote(line, word, rest, limit)
			if !ok {
				lines = append(lines, line)
				line, carried = word, rest

				continue
			}

			lines = append(lines, kept)
			line, carried = moved+" "+word, rest
		}
	}

	if line != "" {
		lines = append(lines, line)
	}

	return lines
}

// breach names the band rule a paragraph's line broke, which is what
// the diagnostic names when the fill does not change the line count.
type breach int

const (
	// held means every line conforms and the paragraph draws nothing.
	held breach = iota
	// overlong is a line past the comment limit with a break available.
	overlong
	// stranded is a line ending on a word that belongs with the line below.
	stranded
	// underfilled is a line short of the minimum that could still absorb the next
	// word.
	underfilled
)

// band is the range of widths a comment line may close inside: from minimum
// up to limit, and for a paragraph of one line, up to overrun past the limit.
type band struct {
	minimum int
	limit   int
	overrun int
}

// banded reports the first band rule one of the paragraph's lines breaks,
// counting each width the way fill counts one. The last line only holds
// the remainder, so nothing but the limit applies to it, and a single word
// wider than the limit has nowhere to break and stands wherever it is.
//
// A paragraph of one line is given the band's overrun past the limit. Filling
// it would only hand its last word down to a line of its own, and where
// a longer paragraph headed that way can move an earlier break left inside
// the band to give the last line company, a single line has no break to move.
func (p paragraph) banded(first, rest int, b band) breach {
	ceiling := b.limit
	if len(p.lines) == 1 {
		ceiling += b.overrun
	}

	for i, line := range p.lines {
		carried := rest
		if i == 0 {
			carried = first
		}

		width := carried + utf8.RuneCountInString(line)
		if width > ceiling {
			if len(strings.Fields(line)) > 1 {
				return overlong
			}

			continue
		}
		if i == len(p.lines)-1 {
			continue
		}

		next := strings.Fields(p.lines[i+1])
		if len(next) == 0 {
			continue
		}
		if _, _, ok := demote(line, next[0], rest, b.limit); ok {
			return stranded
		}
		if width < b.minimum && width+1+utf8.RuneCountInString(opening(next)) <= b.limit {
			return underfilled
		}
	}

	return held
}

// opening returns the least a line above could absorb from the words below it:
// the dangling words the line opens with, together with the first word
// that is not one. The bare first word would be too strict a test — greedy
// hands a lone dangling word straight back through demote, so a line the fill
// closed that way could never satisfy it.
func opening(words []string) string {
	n := 0
	for n < len(words) && dangling[strings.ToLower(words[n])] {
		n++
	}
	if n < len(words) {
		n++
	}

	return strings.Join(words[:n], " ")
}

// demote splits a full line into what it keeps and the run of dangling words
// at its end that should go down with next instead of closing the line.
// It reports false when there is nothing to move, when moving would leave
// the line empty, and when the moved words would not fit alongside next.
func demote(line, next string, carried, limit int) (kept, moved string, ok bool) {
	words := strings.Fields(line)

	keep := len(words)
	for keep > 1 && dangling[strings.ToLower(words[keep-1])] {
		keep--
	}
	if keep == len(words) {
		return "", "", false
	}

	moved = strings.Join(words[keep:], " ")
	if carried+utf8.RuneCountInString(moved)+1+utf8.RuneCountInString(next) > limit {
		return "", "", false
	}

	return strings.Join(words[:keep], " "), moved, true
}

// indentBefore returns the whitespace preceding pos on its line, reporting
// false when anything else does. A comment with code before it is a remark
// about that code and cannot leave the line.
func indentBefore(src []byte, file *token.File, pos token.Pos) (string, bool) {
	prefix := lineUpTo(src, file, pos)
	indent := indentOf(prefix)

	return indent, len(indent) == len(prefix)
}
