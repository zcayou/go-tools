package testlayout_test

import (
	"github.com/golangci/plugin-module-register/register"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/testlayout"
)

var _ = Describe("Plugin", func() {
	It("builds from an absent settings block", func() {
		plugin, err := testlayout.New(nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("builds from an empty settings block", func() {
		plugin, err := testlayout.New(map[string]any{})

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("rejects an unknown settings key rather than ignoring it", func() {
		_, err := testlayout.New(map[string]any{"dedicated-test-dirs": []string{"integration"}})

		Expect(err).To(MatchError(ContainSubstring("dedicated-test-dirs")))
	})

	It("rejects a malformed pattern rather than leaving it matching nothing", func() {
		_, err := testlayout.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{"[_test.go"}},
		})

		Expect(err).To(MatchError(ContainSubstring("blackbox.patterns")))
	})

	It("rejects a malformed helper pattern rather than leaving it matching nothing", func() {
		_, err := testlayout.New(map[string]any{
			"blackbox": map[string]any{"helper-patterns": []string{"[helpers_test.go"}},
		})

		Expect(err).To(MatchError(ContainSubstring("blackbox.helper-patterns")))
	})

	It("rejects a malformed adapter pattern rather than leaving it matching nothing", func() {
		_, err := testlayout.New(map[string]any{"adapter-patterns": []string{"[suite_test.go"}})

		Expect(err).To(MatchError(ContainSubstring("adapter-patterns")))
	})

	It("rejects a pattern naming the source twice, which it cannot resolve", func() {
		_, err := testlayout.New(map[string]any{
			"whitebox": map[string]any{"patterns": []string{"<source>_<source>_test.go"}},
		})

		Expect(err).To(MatchError(ContainSubstring("may appear once")))
	})

	It("rejects a glob wrapped around the source token, which is matched literally", func() {
		_, err := testlayout.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{"*_<source>_test.go"}},
		})

		Expect(err).To(MatchError(ContainSubstring("cannot be combined with a glob")))
	})

	It("rejects a pattern holding a path, since it matches against a file name", func() {
		_, err := testlayout.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{"pkg/<source>_test.go"}},
		})

		Expect(err).To(MatchError(ContainSubstring("no path separator")))
	})

	It("rejects an empty adapter-patterns, which no file could satisfy", func() {
		_, err := testlayout.New(map[string]any{"adapter-patterns": []string{}})

		Expect(err).To(MatchError(ContainSubstring("adapter-patterns is empty")))
	})

	It("rejects both categories being disallowed, which no test file could satisfy", func() {
		_, err := testlayout.New(map[string]any{
			"whitebox": map[string]any{"allowed": false},
			"blackbox": map[string]any{"allowed": false},
		})

		Expect(err).To(MatchError(ContainSubstring("neither whitebox nor blackbox is allowed")))
	})

	It("rejects a category allowed under no name at all", func() {
		_, err := testlayout.New(map[string]any{
			"blackbox": map[string]any{"patterns": []string{}, "helper-patterns": []string{}},
		})

		Expect(err).To(MatchError(ContainSubstring("blackbox is allowed but no pattern permits any name")))
	})

	It("allows a disallowed category to name nothing, since nothing is what it permits", func() {
		_, err := testlayout.New(map[string]any{
			"whitebox": map[string]any{"patterns": []string{}, "helper-patterns": []string{}},
		})

		Expect(err).NotTo(HaveOccurred())
	})

	It("exposes exactly the analyzer golangci-lint should run", func() {
		plugin, err := testlayout.New(nil)
		Expect(err).NotTo(HaveOccurred())

		analyzers, err := plugin.BuildAnalyzers()

		Expect(err).NotTo(HaveOccurred())
		Expect(analyzers).To(HaveLen(1))
		Expect(analyzers[0].Name).To(Equal(testlayout.Name))
	})

	It("asks for syntax-only loading", func() {
		plugin, err := testlayout.New(nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(plugin.GetLoadMode()).To(Equal(register.LoadModeSyntax))
	})
})
