package zassert_test

import (
	"errors"

	"github.com/golangci/plugin-module-register/register"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/filepattern"
	"github.com/zcayou/go-tools/golangci/zassert"
)

var _ = Describe("Plugin", func() {
	It("builds from an absent settings block", func() {
		plugin, err := zassert.New(nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("builds from an empty settings block", func() {
		plugin, err := zassert.New(map[string]any{})

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("rejects an unknown settings key rather than ignoring it", func() {
		_, err := zassert.New(map[string]any{"build-tag": []string{"integration"}})

		Expect(err).To(And(
			BeAssignableToTypeOf(&zassert.ConfigError{}),
			HaveField("Setting", BeZero()),
			HaveField("Problem", zassert.ProblemUndecodableSettings),
		))
		Expect(errors.Unwrap(err)).To(MatchError(ContainSubstring(`unknown field "build-tag"`)))
	})

	It("builds with build tags for the api patterns to resolve under", func() {
		_, err := zassert.New(map[string]any{"api": []string{"./..."}, "build-tags": []string{"integration"}})

		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects an explicitly empty api, which would judge no package", func() {
		_, err := zassert.New(map[string]any{"api": []string{}})

		Expect(err).To(beConfigError(zassert.SettingAPI, zassert.ProblemEmptyAPI))
	})

	It("builds with an explicit api without resolving it, which is the run's to do", func() {
		chdir(anchorlessDir())

		_, err := zassert.New(map[string]any{"api": []string{"./nowhere/..."}})

		Expect(err).NotTo(HaveOccurred())
	})

	DescribeTable("rejects a rendering pattern the file-name grammar refuses",
		func(pattern string, problem filepattern.Problem) {
			_, err := zassert.New(map[string]any{"rendering-patterns": []string{"<source>_test.go", pattern}})

			Expect(err).To(And(
				beConfigError(zassert.SettingRenderingPatterns, zassert.ProblemMalformedRenderingPattern),
				HaveField("Pattern", pattern),
			))
			cause, ok := errors.AsType[*filepattern.Error](err)
			Expect(ok).To(BeTrue())
			Expect(cause.Problem).To(Equal(problem))
		},
		Entry("a path separator", "pkg/<source>_test.go", filepattern.ProblemPathSeparator),
		Entry("a glob around <source>", "*_<source>_test.go", filepattern.ProblemGlobAroundSource),
		Entry("<source> twice", "<source>_<source>_test.go", filepattern.ProblemRepeatedSource),
	)

	DescribeTable("rejects a rendering pattern holding no <source>, which names no source to pin",
		func(pattern string) {
			_, err := zassert.New(map[string]any{"rendering-patterns": []string{pattern}})

			Expect(err).To(And(
				beConfigError(zassert.SettingRenderingPatterns, zassert.ProblemSourcelessRenderingPattern),
				HaveField("Pattern", pattern),
			))
			Expect(errors.Unwrap(err)).To(Succeed())
		},
		Entry("a plain name", "errors_test.go"),
		Entry("a glob", "*_test.go"),
	)

	It("accepts an explicitly empty rendering-patterns, under which no file pins a rendering", func() {
		_, err := zassert.New(map[string]any{"rendering-patterns": []string{}})

		Expect(err).NotTo(HaveOccurred())
	})

	It("exposes exactly the analyzer golangci-lint should run", func() {
		plugin, err := zassert.New(nil)
		Expect(err).NotTo(HaveOccurred())

		analyzers, err := plugin.BuildAnalyzers()

		Expect(err).NotTo(HaveOccurred())
		Expect(analyzers).To(HaveLen(1))
		Expect(analyzers[0].Name).To(Equal(zassert.Name))
	})

	It("asks for syntax-only loading", func() {
		plugin, err := zassert.New(nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(plugin.GetLoadMode()).To(Equal(register.LoadModeTypesInfo))
	})
})
