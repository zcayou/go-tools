package filepattern_test

import (
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/filepattern"
)

var _ = Describe("Pattern", func() {
	srcs := filepattern.Sources{"cache.go", "fs_linux.go", "doc.go"}

	It("matches a glob against the name alone", func() {
		p := mustCompile("*_helpers_test.go")

		Expect(p.Matches("store_helpers_test.go", nil)).To(BeTrue())
		Expect(p.Matches("store_test.go", nil)).To(BeFalse())
		Expect(p.NamesSource()).To(BeFalse())
	})

	It("matches the source token only against a source beside the file", func() {
		p := mustCompile("<source>_test.go")

		Expect(p.Matches("cache_test.go", srcs)).To(BeTrue())
		Expect(p.Matches("store_test.go", srcs)).To(BeFalse())
		Expect(p.NamesSource()).To(BeTrue())
	})

	It("takes a platform variant as the source it splits", func() {
		Expect(mustCompile("<source>_test.go").Matches("fs_test.go", srcs)).To(BeTrue())
	})

	It("says which source a name resolves the token to, whether or not it exists", func() {
		p := mustCompile("spec_<source>_test.go")

		base, ok := p.Source("spec_store_test.go")
		Expect(ok).To(BeTrue())
		Expect(base).To(Equal("store"))
		_, ok = p.Source("store_test.go")
		Expect(ok).To(BeFalse())
		_, ok = p.Source("spec__test.go")
		Expect(ok).To(BeFalse())
	})

	It("lists a set's patterns as they were configured", func() {
		set, err := filepattern.NewSet([]string{"<source>_test.go", "helpers_test.go"})

		Expect(err).NotTo(HaveOccurred())
		Expect(set.String()).To(Equal("<source>_test.go, helpers_test.go"))
		Expect(set.Matches("helpers_test.go", srcs)).To(BeTrue())
	})

	DescribeTable("rejects a pattern no file name could match",
		func(text string, problem filepattern.Problem, message string) {
			_, err := filepattern.New(text)

			Expect(err).To(MatchError(&filepattern.Error{Pattern: text, Problem: problem}))
			Expect(err).To(MatchError(message))
		},
		Entry("a path separator", "pkg/<source>_test.go", filepattern.ProblemPathSeparator,
			`pattern "pkg/<source>_test.go": matched against a file name, which holds no path separator`),
		Entry("a glob around the source token", "*_<source>_test.go", filepattern.ProblemGlobAroundSource,
			`pattern "*_<source>_test.go": <source> is matched literally, so it cannot be combined with a glob`),
		Entry("a repeated source token", "<source>_<source>_test.go", filepattern.ProblemRepeatedSource,
			`pattern "<source>_<source>_test.go": <source> may appear once`),
	)

	It("rejects a malformed glob, carrying what path.Match said", func() {
		_, err := filepattern.New("[_test.go")

		Expect(err).To(MatchError(path.ErrBadPattern))
		Expect(err).To(MatchError(`pattern "[_test.go": syntax error in pattern`))
	})

	DescribeTable("renders what it can of a value New did not make",
		func(err *filepattern.Error, message string) {
			Expect(err).To(MatchError(message))
		},
		Entry("a malformed glob, with nothing said",
			&filepattern.Error{Pattern: "[x", Problem: filepattern.ProblemMalformedGlob},
			`pattern "[x": malformed glob`),
		Entry("a problem outside the enum", &filepattern.Error{Pattern: "x"}, `pattern "x": Problem(0)`),
	)

	It("names each problem", func() {
		Expect([]string{
			filepattern.ProblemPathSeparator.String(),
			filepattern.ProblemMalformedGlob.String(),
			filepattern.ProblemGlobAroundSource.String(),
			filepattern.ProblemRepeatedSource.String(),
		}).To(Equal([]string{"path-separator", "malformed-glob", "glob-around-source", "repeated-source"}))
	})
})

func mustCompile(text string) filepattern.Pattern {
	GinkgoHelper()

	p, err := filepattern.New(text)
	Expect(err).NotTo(HaveOccurred())
	return p
}
