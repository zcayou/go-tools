package deadcode_test

import (
	"github.com/golangci/plugin-module-register/register"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/deadcode"
)

var _ = Describe("Plugin", func() {
	DescribeTable("accepted settings",
		func(settings any) {
			plugin, err := deadcode.New(settings)

			Expect(err).NotTo(HaveOccurred())
			Expect(plugin).NotTo(BeNil())
		},
		Entry("absent", nil),
		Entry("empty", map[string]any{}),
		Entry("patterns", map[string]any{"patterns": []string{"./cmd/..."}}),
		Entry("build tags", map[string]any{"build-tags": []string{"integration"}}),
		Entry("tests off", map[string]any{"tests": false}),
		Entry("api alone defaults its exemption", map[string]any{"api": []string{"./pkg/..."}}),
		Entry("api with exemptions", map[string]any{
			"api":        []string{"./pkg/..."},
			"api-exempt": []string{"method", "func", "type", "const", "var", "interface-method"},
		}),
	)

	DescribeTable("rejected settings",
		func(settings any, message string) {
			_, err := deadcode.New(settings)

			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("an unknown key is a typo, not a no-op",
			map[string]any{"pattern": []string{"./..."}}, "pattern"),
		Entry("an empty pattern list means the packages in it, of which there are none",
			map[string]any{"patterns": []string{}}, "patterns is empty"),
		Entry("exemptions with nothing to exempt",
			map[string]any{"api-exempt": []string{"methods"}}, "api-exempt requires api"),
		Entry("an unknown exemption",
			map[string]any{"api": []string{"./..."}, "api-exempt": []string{"nonsense"}},
			`unknown api-exempt "nonsense"`),
	)

	It("names the decoding failure once", func() {
		// register.DecodeSettings already says what it was doing, and golangci-lint
		// prefixes the plugin name on top of that.
		_, err := deadcode.New(map[string]any{"nope": true})

		Expect(err.Error()).To(Equal(`decoding settings: json: unknown field "nope"`))
	})

	It("builds without touching the filesystem", func() {
		// golangci-lint builds every configured plugin whether the run enables
		// it or not, and a failure here aborts configuration loading for unrelated
		// commands too.
		chdir(anchorlessDir())

		plugin, err := deadcode.New(nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("asks for syntax-only loading, because the engine does its own typed load", func() {
		plugin, err := deadcode.New(nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(plugin.GetLoadMode()).To(Equal(register.LoadModeSyntax))
	})
})
