package zassert_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zassert"
)

var _ = Describe("ConfigError", func() {
	It("renders settings that do not decode with what the decoder said", func() {
		_, err := zassert.New(map[string]any{"bogus": true})

		Expect(err).To(MatchError(`zassert: decoding settings: json: unknown field "bogus"`))
	})

	It("renders an empty api", func() {
		_, err := zassert.New(map[string]any{"api": []string{}})

		Expect(err).To(MatchError(
			"zassert: api: an empty list names no package; leave the key out to judge every API package"))
	})

	It("renders a malformed rendering pattern with what the grammar said", func() {
		_, err := zassert.New(map[string]any{"rendering-patterns": []string{"pkg/<source>_test.go"}})

		Expect(err).To(MatchError(`zassert: rendering-patterns: pattern "pkg/<source>_test.go": ` +
			"matched against a file name, which holds no path separator"))
	})

	It("renders a rendering pattern holding no <source>", func() {
		_, err := zassert.New(map[string]any{"rendering-patterns": []string{"*_test.go"}})

		Expect(err).To(MatchError(`zassert: rendering-patterns: pattern "*_test.go" holds no <source>, ` +
			"so it names no source file a test file could pin the rendering of"))
	})

	It("renders an api pattern the module does not resolve with what the loader said", func() {
		dir := apiscopeDir()
		chdir(dir)
		analyzer, err := zassert.NewAnalyzer(zassert.Settings{API: []string{"./empty/..."}})
		Expect(err).NotTo(HaveOccurred())

		_, err = runPass(analyzer, loadPasses(dir)[0], dir)

		Expect(err).To(MatchError("zassert: api: pattern ./empty/... matched no packages"))
	})

	It("renders a missing module root with where the walk started", func() {
		dir := apiscopeDir()
		pkgs := loadPasses(dir)
		analyzer, err := zassert.NewAnalyzer(zassert.Settings{API: []string{"./narrow"}})
		Expect(err).NotTo(HaveOccurred())
		anchorless := anchorlessDir()
		chdir(anchorless)

		_, err = runPass(analyzer, pkgs[0], dir)

		Expect(err).To(MatchError(
			"zassert: api: resolving its patterns needs the module root: no go.mod at or above " + anchorless))
	})

	DescribeTable("renders what it can of a value no setting produced",
		func(err *zassert.ConfigError, message string) {
			Expect(err).To(MatchError(message))
		},
		Entry("undecodable, with nothing said",
			&zassert.ConfigError{Problem: zassert.ProblemUndecodableSettings}, "zassert: settings do not decode"),
		Entry("no module root, with nothing said",
			&zassert.ConfigError{Setting: zassert.SettingAPI, Problem: zassert.ProblemNoModuleRoot},
			"zassert: api: resolving its patterns needs the module root"),
		Entry("a malformed rendering pattern, with nothing said",
			&zassert.ConfigError{
				Setting: zassert.SettingRenderingPatterns, Problem: zassert.ProblemMalformedRenderingPattern,
				Pattern: "x",
			},
			`zassert: rendering-patterns: pattern "x": malformed-rendering-pattern`),
		Entry("an unresolved api pattern, with nothing said",
			&zassert.ConfigError{
				Setting: zassert.SettingAPI, Problem: zassert.ProblemUnresolvedAPIPattern, Pattern: "./x",
			},
			`zassert: api: pattern "./x": unresolved-api-pattern`),
		Entry("a problem outside the enum", &zassert.ConfigError{}, "zassert: Setting(0): Problem(0)"),
	)

	It("names each setting by its key", func() {
		Expect(zassert.SettingAPI.String()).To(Equal("api"))
		Expect(zassert.SettingRenderingPatterns.String()).To(Equal("rendering-patterns"))

		var unset zassert.Setting
		Expect(unset.String()).To(Equal("Setting(0)"))
	})

	It("names each problem by a token of its own", func() {
		Expect(zassert.ProblemUndecodableSettings.String()).To(Equal("undecodable-settings"))
		Expect(zassert.ProblemEmptyAPI.String()).To(Equal("empty-api"))
		Expect(zassert.ProblemMalformedRenderingPattern.String()).To(Equal("malformed-rendering-pattern"))
		Expect(zassert.ProblemSourcelessRenderingPattern.String()).To(Equal("sourceless-rendering-pattern"))
		Expect(zassert.ProblemNoModuleRoot.String()).To(Equal("no-module-root"))
		Expect(zassert.ProblemUnresolvedAPIPattern.String()).To(Equal("unresolved-api-pattern"))

		var unset zassert.Problem
		Expect(unset.String()).To(Equal("Problem(0)"))
	})
})
