package deadcode_test

import (
	"github.com/golangci/plugin-module-register/register"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/deadcode"
	"github.com/zcayou/go-tools/golangci/deadcode/engine"
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
		Entry("api with a generic rooting", map[string]any{
			"api":          []string{"./pkg/..."},
			"api-generics": "instantiated",
		}),
		Entry("api with its tests standing in for consumers", map[string]any{
			"api":                []string{"./pkg/..."},
			"api-test-consumers": true,
		}),
		Entry("vocabulary names", map[string]any{"vocabulary-names": true}),
		Entry("test-facing under the default tests", map[string]any{"test-facing": []string{"./plugins/test/..."}}),
		Entry("roots", map[string]any{"roots": []string{"tools/*.go"}}),
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
		Entry("an empty exemption list with nothing to exempt",
			map[string]any{"api-exempt": []string{}}, "api-exempt requires api"),
		Entry("an unknown exemption",
			map[string]any{"api": []string{"./..."}, "api-exempt": []string{"nonsense"}},
			`unknown api-exempt "nonsense"`),
		Entry("a generic rooting with nothing to root",
			map[string]any{"api-generics": "instantiated"}, "api-generics requires api"),
		Entry("an unknown generic rooting",
			map[string]any{"api": []string{"./..."}, "api-generics": "monomorphized"},
			`unknown api-generics "monomorphized"`),
		Entry("tests standing in for the consumers of no api",
			map[string]any{"api-test-consumers": true}, "api-test-consumers requires api"),
		Entry("tests standing in for consumers with the masked view turned off",
			map[string]any{"api": []string{"./pkg/..."}, "tests": false, "api-test-consumers": true},
			"api-test-consumers requires tests"),
		Entry("test-facing with the masked view it speaks to turned off",
			map[string]any{"tests": false, "test-facing": []string{"./plugins/test/..."}},
			"test-facing requires tests"),
		Entry("an empty test-facing list means the packages in it, of which there are none",
			map[string]any{"test-facing": []string{}}, "test-facing is empty"),
		Entry("an empty roots list means the files in it, of which there are none",
			map[string]any{"roots": []string{}}, "roots is empty"),
	)

	DescribeTable("api exemptions",
		func(settings map[string]any, expected []engine.Kind) {
			Expect(engineConfig(settings).APIExempt).To(Equal(expected))
		},
		Entry("default to method when the key is absent",
			map[string]any{"api": []string{"./pkg/..."}}, []engine.Kind{engine.KindMethod}),
		Entry("are nothing when the list is written out empty",
			map[string]any{"api": []string{"./pkg/..."}, "api-exempt": []string{}}, []engine.Kind{}),
		Entry("are the listed kinds otherwise",
			map[string]any{"api": []string{"./pkg/..."}, "api-exempt": []string{"func", "var"}},
			[]engine.Kind{engine.KindFunc, engine.KindVar}),
	)

	It("hands the engine tests standing in for the api's consumers", func() {
		config := engineConfig(map[string]any{"api": []string{"./pkg/..."}, "api-test-consumers": true})

		Expect(config.APITestConsumers).To(BeTrue())
	})

	It("hands the engine the vocabulary-names credit", func() {
		Expect(engineConfig(map[string]any{"vocabulary-names": true}).VocabularyNames).To(BeTrue())
	})

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

// engineConfig builds the plugin from settings and returns the configuration
// it hands the engine.
func engineConfig(settings map[string]any) engine.Config {
	GinkgoHelper()

	plugin, err := deadcode.New(settings)
	Expect(err).NotTo(HaveOccurred())
	built, ok := plugin.(*deadcode.Plugin)
	Expect(ok).To(BeTrue())
	return built.EngineConfig()
}
