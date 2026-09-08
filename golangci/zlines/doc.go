// Package zlines reports line breaks a declaration should not have, and line
// breaks it should.
//
// A line-length linter such as lll polices lines that are too long. It has
// nothing to say about a declaration that was wrapped over several lines
// and would now sit comfortably inside the limit on one — after a rename, after
// a parameter was dropped, or because it was written wrapped out of habit. Nor
// about the opposite, a body folded onto the line its signature ends on, where
// a reader going down a file's declarations does not expect to find code. This
// linter reports both, and offers a fix for each.
//
// # Rules
//
// Each diagnostic names the rule that produced it as the first word of its
// message, so a consumer can exclude one rule without silencing the linter:
//
//   - signature-wrap: a function signature is spread over several lines but
//     fits inside the limit on one.
//   - body-collapse: a function body is written on the line its signature ends
//     on rather than lines of its own.
//   - comment-wrap: a comment paragraph breaks a line outside the band its
//     settings draw.
//
// # What counts as the line
//
// The width compared against the limit is the width of the line the fix would
// produce, which is more than the signature: it takes in whatever precedes
// the func keyword on its line, and whatever follows the signature on the last
// one — the opening brace, and a trailing comment if there is one. A body
// written on that line is the one thing left out, because body-collapse fixes
// it onto lines of its own, and a width that counted it would report
// on the second run what it had nothing to say about on the first. Width
// is counted in characters, and a tab counts as one.
//
// # When a wrap stands
//
// A wrap is left alone, with no diagnostic, whenever joining it would lose
// something or would not survive gofmt:
//
//   - A comment inside the signature that one line cannot hold, or would put
//     the wrong construction on. A line comment would swallow the rest
//     of the joined line. A comment spanning lines cannot sit on one.
//     And a block comment given a line of its own documents the parameter
//     beneath it, which joining would attach to the parameter above instead —
//     the code is the same either way, but the comment no longer says what
//     it said.
//
//     A block comment written alongside a parameter says which one it is about
//     without the line break, so it is carried along and the wrap is reported.
//
//   - A type gofmt insists on spreading over several lines however it was
//     written, such as a struct type holding more than one field. The fix
//     is offered only when gofmt agrees the result is one line, which is what
//     makes it stable: running the formatter afterwards cannot undo it.
//
// Neither case is reported without a fix, because in both the wrap is the right
// way to write the declaration.
//
// # A body of its own
//
// body-collapse takes no account of the line length: however short the line,
// a declaration read at a glance should show a signature, not a signature
// with the code folded in beside it. The rule holds for function declarations,
// which are the whole of what this linter looks at. A function literal handed
// to a call is left alone, since one line is often where such a body reads
// best.
//
// A body declaring no statements is not collapsed onto anything. gofmt writes an empty body as {}
// and there is nothing in it to move, so func noop() {}, a body holding only
// a comment, and one holding only a bare semicolon all stand.
//
// The fix renders the body through gofmt, which is what puts a body of several
// statements back one to a line. Unlike a wrap that stands, the diagnostic does
// not wait on that rendering: there is no line length at which a folded body
// is the right way to write the declaration. What is handed to gofmt is source
// the rule has already broken over lines of its own, so the formatter has no
// way to decline it; the report is written to carry no fix if one ever comes
// back, because a call out to a formatter is not a place to be certain.
//
// # Filling a comment
//
// A comment line may stop anywhere between comment-min-length
// and comment-length, and a paragraph whose breaks all land inside that band
// is left alone however far they sit from the ones a fill would choose — which
// is what keeps one edited word from rewrapping every line below it. A line
// past the limit is reported unless it is a single word with nowhere to break,
// and a line short of the minimum is reported only while the line below opens
// with something it could still have taken: a break the words force stands,
// however short it leaves the line. The last line of a paragraph only holds
// the remainder, so no minimum applies to it.
//
// A paragraph of one line is given comment-overrun columns past the limit.
// Filling such a line only hands its last word down to a line of its own,
// and where a longer paragraph headed that way has a way out inside the limit —
// an earlier break moved left within the band gives the last line company —
// a single line has no break to move. So it may run a little past the limit
// rather than split, and past the overrun it is filled like any other. A longer
// paragraph is never given the overrun, whatever its last line would be left
// holding.
//
// The fix fills the paragraph greedily to comment-length and writes the line
// breaks that gives it, so the accepted forms are many but the fixed one
// is always the same. The line breaks inside a paragraph are taken to carry
// nothing, because the break that carries something is the blank //-line:
// it ends the paragraph, and no fill ever runs across one. Width is counted
// the way "What counts as the line" counts it, so a comment nested several
// tabs deep fills to the limit in characters and lands past it in columns.
//
// The rule reads only // comments that nothing but whitespace precedes on their
// line. A comment sitting after code is a remark about that code and cannot
// leave the line; a /* */ block is left alone entirely.
//
// A list item is prose too, and fills the same way — inside the marker it was
// written with, its continuation lines hanging under that. Each item is a run
// of its own: the item above is no more part of it than the sentence above
// the list, and nothing is ever filled from one item into another. The marker
// the file already carries is reproduced rather than chosen, so the rule
// changes line breaks and leaves indentation to gofmt. A list is read where
// gofmt reads one, which is where the marker is still indented once the space
// every prose line carries comes off; a dash at the margin opens a sentence
// rather than an item.
//
// What is left keeps the shape it was given: a heading, an indented code block,
// a link definition, a directive such as //go:build, the legacy +build line
// that goes with it, the marker that makes a file generated, a comment written
// without the space after //, and a line closing on a brace or a semicolon,
// which is code someone commented out rather than a sentence. An indented line
// is a continuation where an item is open above it and a code block anywhere
// else, which is the distinction gofmt draws as well.
//
// # Comments addressed to a tool
//
// A comment line can be meant for a tool rather than a reader — a +kubebuilder
// marker, a directive quoted in prose — and moving it changes what it does.
// comment-exempt lists the prefixes that mark such a line. A line opening
// with one is left where it was put and does not join the sentence above it,
// and a paragraph carrying one anywhere is left whole, because filling could
// put the word at the start of a line and make it live.
//
// The nolint prefix holds on top of the setting, for the reason the
// alwaysExempt declaration in plugin.go gives at length: golangci-lint reads
// such a token out of any comment line, and filling is what could move a quoted
// one onto a line of its own, where it goes live.
//
// # Where a line breaks
//
// A line is not left ending on a word whose whole business is joining
// it to what follows — an article, a preposition, a conjunction. Filling
// greedily would carry such a word up to the margin and strand it there, so
// it goes down to the word it belongs with instead, and the line comes out
// shorter than the limit would allow. The band does not loosen this: a break
// after such a word is reported at any width, except where no line below
// could hold it beside the word it belongs with, which is the one place
// the fill leaves such a break itself. The fill bends for nothing else: a word
// too wide for the limit still takes a line of its own and overruns it, there
// being nowhere to break it.
//
// # Settings
//
// The limit signature-wrap holds a joined line to comes from line-length, which
// defaults to [DefaultLineLength]. Comments are filled to comment-length
// instead, which defaults to [DefaultCommentLength]: prose and code are wrapped
// to different measures for different reasons, and a repository that holds code
// to 120 columns rarely wants its sentences that wide. The band's lower edge
// is comment-min-length, which an absent key derives
// as [DefaultCommentUnderfill] short of the length so that one key moves both
// edges; a minimum equal to the length asks for the exact fill back.
// A paragraph of one line may run comment-overrun columns past the length,
// which defaults to [DefaultCommentOverrun]; zero holds it to the length like
// any other. Every rule is enforced unless a repository says otherwise,
// and each has a key of its own to say it with:
//
//	settings:
//	  custom:
//	    zlines:
//	      type: module
//	      settings:
//	        line-length: 160
//	        comment-length: 80
//	        comment-min-length: 70
//	        comment-overrun: 10
//	        signature-wrap: false
//	        body-collapse: false
//	        comment-wrap: false
//	        comment-exempt: ['+kubebuilder']
//
// The line length wants to be the one the repository already holds its lines
// to, since a signature joined past it only trades one complaint for another.
//
// # Entry points
//
// [New] is the golangci-lint plugin constructor and is what
// github.com/zcayou/go-tools/golangci registers. [NewAnalyzer] builds
// the underlying [analysis.Analyzer] directly and is the entry point for tests
// and for any other go/analysis driver.
//
// # Load mode
//
// Every rule is syntactic, so the linter runs under [register.LoadModeSyntax]
// and never forces type checking on a consumer. It does read each file's source
// alongside the syntax tree, because the exact text a fix has to reproduce
// is not recoverable from the tree alone.
//
// # Suppression
//
// golangci-lint applies the nolint directive centrally, matched on the linter
// name, so a "nolint:zlines" comment suppresses a finding: on the line the
// signature opens for signature-wrap, on the line the body opens for
// body-collapse, which is the last line of the signature, and on the first line
// of the paragraph for comment-wrap. This linter does not interpret such
// directives itself, and the directive is spelled out rather than written here
// for the reason "Comments addressed to a tool" gives.
// A directive placed inside the signature is a line comment, so it also stops
// a signature-wrap diagnostic by keeping the wrap: see "When a wrap stands".
//
// [analysis.Analyzer]: https://pkg.go.dev/golang.org/x/tools/go/analysis#Analyzer
// [register.LoadModeSyntax]: https://pkg.go.dev/github.com/golangci/plugin-module-register/register#LoadModeSyntax
package zlines
