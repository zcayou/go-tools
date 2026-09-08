package zlines_test

import (
	"github.com/golangci/plugin-module-register/register"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/zlines"
)

// Each setting is put to a source the rule it governs has something to say
// about, because the only thing that tells a setting that was read from one
// that was decoded and dropped is a run.
const (
	wrappedSignature = "package p\n" +
		"\n" +
		"func wrapped(\n" +
		"\tcount int,\n" +
		") error {\n" +
		"\treturn nil\n" +
		"}\n"

	collapsedBody = "package p\n" +
		"\n" +
		"func collapsed() int { return 1 }\n"

	shortComment = "package p\n" +
		"\n" +
		"// a comment\n" +
		"// wrapped short\n" +
		"type thing struct{}\n"

	markedComment = "package p\n" +
		"\n" +
		"// +kubebuilder:validation:Required\n" +
		"// and a line of prose under it\n" +
		"type marked struct{}\n"

	inBandComment = "package p\n" +
		"\n" +
		"// the band tolerates a break placed anywhere inside it, so editing one word\n" +
		"// did not cascade a rewrap through the paragraph.\n" +
		"type banded struct{}\n"
)

var _ = Describe("Plugin", func() {
	It("builds from an absent settings block", func() {
		plugin, err := zlines.New(nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("takes the line length from the settings block", func() {
		Expect(analyze(analyzerFrom(nil), wrappedSignature).messages()).To(HaveLen(1))
		Expect(analyze(analyzerFrom(map[string]any{"line-length": 20}), wrappedSignature).messages()).
			To(BeEmpty())
	})

	It("takes the signature-wrap rule out of the run when asked to", func() {
		Expect(analyze(analyzerFrom(map[string]any{"signature-wrap": false}), wrappedSignature).messages()).
			To(BeEmpty())
	})

	It("takes the body-collapse rule out of the run when asked to", func() {
		Expect(analyze(analyzerFrom(nil), collapsedBody).messages()).To(HaveLen(1))
		Expect(analyze(analyzerFrom(map[string]any{"body-collapse": false}), collapsedBody).messages()).
			To(BeEmpty())
	})

	It("holds comments to a width of their own, apart from the line length", func() {
		settings := map[string]any{"line-length": 120, "comment-length": 18}

		Expect(analyze(analyzerFrom(nil), shortComment).messages()).To(HaveLen(1))
		Expect(analyze(analyzerFrom(settings), shortComment).messages()).To(BeEmpty())
	})

	It("takes the comment minimum from the settings block", func() {
		settings := map[string]any{"comment-min-length": 80}

		Expect(analyze(analyzerFrom(nil), inBandComment).messages()).To(BeEmpty())
		Expect(analyze(analyzerFrom(settings), inBandComment).messages()).To(HaveLen(1))
	})

	It("takes the comment overrun from the settings block", func() {
		settings := map[string]any{"comment-overrun": 0}

		Expect(analyze(analyzerFrom(nil), loneComment).messages()).To(BeEmpty())
		Expect(analyze(analyzerFrom(settings), loneComment).messages()).To(HaveLen(1))
	})

	It("takes the comment-wrap rule out of the run when asked to", func() {
		Expect(analyze(analyzerFrom(map[string]any{"comment-wrap": false}), shortComment).messages()).
			To(BeEmpty())
	})

	It("takes exempt comment prefixes from the settings block", func() {
		settings := map[string]any{"comment-exempt": []string{"+kubebuilder"}}

		Expect(analyze(analyzerFrom(nil), markedComment).messages()).To(HaveLen(1))
		Expect(analyze(analyzerFrom(settings), markedComment).messages()).To(BeEmpty())
	})

	It("decodes every documented key onto the field it belongs to", func() {
		settings, err := register.DecodeSettings[zlines.Settings](map[string]any{
			"line-length":        160,
			"comment-length":     80,
			"comment-min-length": 70,
			"comment-overrun":    10,
			"signature-wrap":     false,
			"body-collapse":      false,
			"comment-wrap":       false,
			"comment-exempt":     []string{"+kubebuilder"},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(settings).To(Equal(zlines.Settings{
			LineLength:       new(160),
			CommentLength:    new(80),
			CommentMinLength: new(70),
			CommentOverrun:   new(10),
			SignatureWrap:    new(false),
			BodyCollapse:     new(false),
			CommentWrap:      new(false),
			CommentExempt:    []string{"+kubebuilder"},
		}))
	})

	It("rejects a negative comment length rather than reading it as unset", func() {
		_, err := zlines.New(map[string]any{"comment-length": -1})

		Expect(err).To(MatchError(ContainSubstring("comment-length must be positive")))
	})

	It("rejects a comment length of zero rather than defaulting it", func() {
		_, err := zlines.New(map[string]any{"comment-length": 0})

		Expect(err).To(MatchError(ContainSubstring("comment-length must be positive")))
	})

	It("rejects a comment minimum of zero rather than defaulting it", func() {
		_, err := zlines.New(map[string]any{"comment-min-length": 0})

		Expect(err).To(MatchError(ContainSubstring("comment-min-length must be positive")))
	})

	It("rejects a comment minimum past the length, a band nothing could satisfy", func() {
		_, err := zlines.New(map[string]any{"comment-length": 80, "comment-min-length": 90})

		Expect(err).To(MatchError(ContainSubstring("comment-min-length 90 exceeds comment-length 80")))
	})

	It("rejects a negative comment overrun, which would report the fill's own line", func() {
		_, err := zlines.New(map[string]any{"comment-overrun": -1})

		Expect(err).To(MatchError(ContainSubstring("comment-overrun must not be negative")))
	})

	It("rejects an unknown settings key rather than ignoring it", func() {
		_, err := zlines.New(map[string]any{"max-line-length": 160})

		Expect(err).To(MatchError(ContainSubstring("max-line-length")))
	})

	It("rejects a negative line length rather than reading it as unset", func() {
		_, err := zlines.New(map[string]any{"line-length": -1})

		Expect(err).To(MatchError(ContainSubstring("line-length must be positive")))
	})

	It("rejects a line length of zero rather than defaulting it", func() {
		_, err := zlines.New(map[string]any{"line-length": 0})

		Expect(err).To(MatchError(ContainSubstring("line-length must be positive")))
	})

	It("rejects an empty exempt prefix, which would exempt every comment there is", func() {
		_, err := zlines.New(map[string]any{"comment-exempt": []string{""}})

		Expect(err).To(MatchError(ContainSubstring("comment-exempt must not hold an empty prefix")))
	})

	It("exposes exactly the analyzer golangci-lint should run", func() {
		plugin, err := zlines.New(nil)
		Expect(err).NotTo(HaveOccurred())

		analyzers, err := plugin.BuildAnalyzers()

		Expect(err).NotTo(HaveOccurred())
		Expect(analyzers).To(HaveLen(1))
		Expect(analyzers[0].Name).To(Equal(zlines.Name))
	})

	It("asks for syntax-only loading", func() {
		plugin, err := zlines.New(nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(plugin.GetLoadMode()).To(Equal(register.LoadModeSyntax))
	})
})

// analyzerFrom builds the plugin from settings and returns the analyzer
// it exposes, which is the whole of what golangci-lint gets out of a settings
// block.
func analyzerFrom(settings any) *analysis.Analyzer {
	GinkgoHelper()

	plugin, err := zlines.New(settings)
	Expect(err).NotTo(HaveOccurred())

	analyzers, err := plugin.BuildAnalyzers()
	Expect(err).NotTo(HaveOccurred())
	Expect(analyzers).To(HaveLen(1))

	return analyzers[0]
}
