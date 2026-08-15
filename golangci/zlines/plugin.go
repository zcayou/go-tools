package zlines

import (
	"fmt"
	"slices"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

// Name is what golangci-lint knows this linter by: the key under
// linters.settings.custom, the entry in linters.enable, and the token
// a //nolint:zlines directive must carry.
const Name = "zlines"

// DefaultLineLength is the limit in force when the setting is left out.
// It is the default of the lll linter, so a repository that has not settled
// on a line length gets the conventional one.
const DefaultLineLength = 120

// DefaultCommentLength is the width comment paragraphs are filled to when
// the setting is left out. Prose is held to a shorter measure than code,
// and for a different reason: past roughly 75 characters the eye starts to lose
// the line on the way back to the left margin. It is where the standard library
// wraps its doc comments, and where Go repositories tend to land unprompted.
const DefaultCommentLength = 80

// DefaultCommentSlack is how far short of the comment length a line may stop
// when comment-min-length is left out. Deriving the minimum keeps the band
// following whatever length a repository configures, so one key moves both
// of its edges.
const DefaultCommentSlack = 10

// Settings is the decoded linters.settings.custom.zlines.settings block.
//
// Decoding rejects unknown fields, so a misspelled key fails the run rather
// than being silently ignored. The zero value asks for [DefaultLineLength]
// and for every rule to be enforced.
type Settings struct {
	// LineLength is the widest line the repository allows. A wrapped signature
	// is reported only when joining it yields a line no wider than this, so
	// the limit wants to be the one the repository already holds its lines to.
	// An absent key asks for [DefaultLineLength]; the pointer is what tells
	// it from a written-out limit.
	LineLength *int `json:"line-length"`

	// CommentLength is the width comment paragraphs are filled to, on the same
	// terms as LineLength. It is not LineLength: prose and code are wrapped
	// to different measures, for different reasons, and a repository that holds
	// code to 120 rarely wants its sentences that wide.
	CommentLength *int `json:"comment-length"`

	// CommentMinLength is the width a comment line may stop short at without being
	// reported, the lower edge of the band CommentLength closes. Inside the band
	// line breaks are free, which is what keeps one edited word from rewrapping
	// a whole paragraph; below it a line that could still absorb the next word
	// is reported. An absent key derives the minimum as [DefaultCommentSlack]
	// short of the comment length, and a minimum equal to CommentLength asks
	// for the exact fill back.
	CommentMinLength *int `json:"comment-min-length"`

	// SignatureWrap says whether the signature-wrap rule is enforced. An absent
	// key enforces it, so turning the rule off is something a repository has
	// to say; the pointer is what tells an absent key from an explicit false.
	SignatureWrap *bool `json:"signature-wrap"`

	// BodyCollapse says whether the body-collapse rule is enforced, on the same
	// terms as SignatureWrap.
	BodyCollapse *bool `json:"body-collapse"`

	// CommentWrap says whether the comment-wrap rule is enforced, on the same
	// terms as SignatureWrap.
	CommentWrap *bool `json:"comment-wrap"`

	// CommentExempt are prefixes marking comment lines addressed to a tool rather than a reader —
	// "+kubebuilder" for the controller-gen markers, say. A line opening with one is left as written,
	// and so is any paragraph carrying one, since filling could move the token to the start of a line
	// and make it live. The nolint prefix is honored on top of whatever is listed.
	CommentExempt []string `json:"comment-exempt"`
}

// alwaysExempt are the prefixes comment-wrap honors whatever the settings say.
//
// golangci-lint reads a nolint directive out of any comment line, trimming
// leading slashes and spaces before it looks, so a doc comment that quotes one
// is already a directive as far as it is concerned. Filling such a comment can
// leave the token alone on a line, which is the form that suppresses rather than
// merely warning — and the bare word, with no linter named, suppresses every
// linter at once.
var alwaysExempt = []string{"nolint"}

// commentExempt is the full set of prefixes in force.
func (s Settings) commentExempt() []string {
	return slices.Concat(alwaysExempt, s.CommentExempt)
}

// lineLength is the limit in force, which an unset setting leaves
// at [DefaultLineLength].
func (s Settings) lineLength() int {
	if s.LineLength == nil {
		return DefaultLineLength
	}

	return *s.LineLength
}

// commentLength is the width comment paragraphs are filled to, which an unset
// setting leaves at [DefaultCommentLength].
func (s Settings) commentLength() int {
	if s.CommentLength == nil {
		return DefaultCommentLength
	}

	return *s.CommentLength
}

// commentMinLength is the width a comment line may stop short at, which
// an unset setting derives from the comment length in force.
func (s Settings) commentMinLength() int {
	if s.CommentMinLength == nil {
		return max(1, s.commentLength()-DefaultCommentSlack)
	}

	return *s.CommentMinLength
}

// signatureWrap reports whether the signature-wrap rule is enforced, which
// an unset setting leaves on.
func (s Settings) signatureWrap() bool {
	return s.SignatureWrap == nil || *s.SignatureWrap
}

// bodyCollapse reports whether the body-collapse rule is enforced, which
// an unset setting leaves on.
func (s Settings) bodyCollapse() bool {
	return s.BodyCollapse == nil || *s.BodyCollapse
}

// commentWrap reports whether the comment-wrap rule is enforced, which an unset
// setting leaves on.
func (s Settings) commentWrap() bool {
	return s.CommentWrap == nil || *s.CommentWrap
}

// Plugin adapts the analyzer to golangci-lint's module plugin contract.
type Plugin struct {
	settings Settings
}

var _ register.LinterPlugin = (*Plugin)(nil)

// New builds the plugin from the raw settings golangci-lint decoded out
// of the configuration file. It satisfies [register.NewPlugin].
func New(settings any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](settings)
	if err != nil {
		return nil, fmt.Errorf("%s: decoding settings: %w", Name, err)
	}
	// A limit of zero or less is a mistake rather than a way to turn a rule off,
	// which is what the rule's own key is for.
	if s.LineLength != nil && *s.LineLength <= 0 {
		return nil, fmt.Errorf("%s: line-length must be positive, got %d", Name, *s.LineLength)
	}
	if s.CommentLength != nil && *s.CommentLength <= 0 {
		return nil, fmt.Errorf("%s: comment-length must be positive, got %d", Name, *s.CommentLength)
	}
	if s.CommentMinLength != nil && *s.CommentMinLength <= 0 {
		return nil, fmt.Errorf("%s: comment-min-length must be positive, got %d", Name, *s.CommentMinLength)
	}
	// A minimum past the length is a band no paragraph could sit inside.
	if s.CommentMinLength != nil && *s.CommentMinLength > s.commentLength() {
		return nil, fmt.Errorf("%s: comment-min-length %d exceeds comment-length %d",
			Name, *s.CommentMinLength, s.commentLength())
	}
	// Every string opens with the empty prefix, so listing it would exempt every
	// comment there is and leave comment-wrap enforced but silent.
	if slices.Contains(s.CommentExempt, "") {
		return nil, fmt.Errorf("%s: comment-exempt must not hold an empty prefix", Name)
	}

	return &Plugin{settings: s}, nil
}

// BuildAnalyzers returns the single analyzer this linter runs.
func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{NewAnalyzer(p.settings)}, nil
}

// GetLoadMode reports that the rules are syntactic, sparing consumers the cost
// of type checking for this linter.
func (p *Plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}
