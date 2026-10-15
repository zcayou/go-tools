package deadcode_test

import (
	"errors"

	"github.com/golangci/plugin-module-register/register"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/deadcode"
	"github.com/zcayou/go-tools/golangci/deadcode/engine"
	"github.com/zcayou/go-tools/golangci/internal/ruleset"
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
		Entry("enum names", map[string]any{"enum-names": true}),
		Entry("test-facing under the default tests", map[string]any{"test-facing": []string{"./plugins/test/..."}}),
		Entry("roots", map[string]any{"roots": []string{"tools/*.go"}}),
		Entry("every rule off", map[string]any{"default": "none"}),
	)

	It("rejects an unknown key, which is a typo rather than a no-op", func() {
		_, err := deadcode.New(map[string]any{"pattern": []string{"./..."}})

		Expect(err).To(BeAssignableToTypeOf(&engine.ConfigError{}))
		Expect(err).To(HaveField("Problem", engine.ProblemUndecodable))
		Expect(errors.Unwrap(err)).To(MatchError(ContainSubstring(`unknown field "pattern"`)))
	})

	DescribeTable("rejected settings",
		func(settings any, expected *engine.ConfigError) {
			_, err := deadcode.New(settings)

			Expect(err).To(MatchError(expected))
		},
		Entry("an empty pattern list means the packages in it, of which there are none",
			map[string]any{"patterns": []string{}},
			&engine.ConfigError{Setting: engine.SettingPatterns, Problem: engine.ProblemEmpty}),
		Entry("exemptions with nothing to exempt",
			map[string]any{"api-exempt": []string{"methods"}},
			&engine.ConfigError{Setting: engine.SettingAPIExempt, Problem: engine.ProblemRequiresAPI}),
		Entry("an empty exemption list with nothing to exempt",
			map[string]any{"api-exempt": []string{}},
			&engine.ConfigError{Setting: engine.SettingAPIExempt, Problem: engine.ProblemRequiresAPI}),
		Entry("an unknown exemption",
			map[string]any{"api": []string{"./..."}, "api-exempt": []string{"nonsense"}},
			&engine.ConfigError{Setting: engine.SettingAPIExempt, Problem: engine.ProblemUnknownValue, Value: "nonsense"}),
		Entry("a generic rooting with nothing to root",
			map[string]any{"api-generics": "instantiated"},
			&engine.ConfigError{Setting: engine.SettingAPIGenerics, Problem: engine.ProblemRequiresAPI}),
		Entry("an unknown generic rooting",
			map[string]any{"api": []string{"./..."}, "api-generics": "monomorphized"},
			&engine.ConfigError{
				Setting: engine.SettingAPIGenerics, Problem: engine.ProblemUnknownValue, Value: "monomorphized",
			}),
		Entry("tests standing in for the consumers of no api",
			map[string]any{"api-test-consumers": true},
			&engine.ConfigError{Setting: engine.SettingAPITestConsumers, Problem: engine.ProblemRequiresAPI}),
		Entry("tests standing in for consumers with the masked view turned off",
			map[string]any{"api": []string{"./pkg/..."}, "tests": false, "api-test-consumers": true},
			&engine.ConfigError{Setting: engine.SettingAPITestConsumers, Problem: engine.ProblemRequiresTests}),
		Entry("test-facing with the masked view it speaks to turned off",
			map[string]any{"tests": false, "test-facing": []string{"./plugins/test/..."}},
			&engine.ConfigError{Setting: engine.SettingTestFacing, Problem: engine.ProblemRequiresTests}),
		Entry("an empty test-facing list means the packages in it, of which there are none",
			map[string]any{"test-facing": []string{}},
			&engine.ConfigError{Setting: engine.SettingTestFacing, Problem: engine.ProblemEmpty}),
		Entry("an empty roots list means the files in it, of which there are none",
			map[string]any{"roots": []string{}},
			&engine.ConfigError{Setting: engine.SettingRoots, Problem: engine.ProblemEmpty}),
		Entry("inert-field with no api to derive its subjects from",
			map[string]any{"enable": []string{"inert-field"}},
			&engine.ConfigError{Rule: engine.RuleInertField, Problem: engine.ProblemRequiresAPI}),
		Entry("echo-functions while inert-field is off",
			map[string]any{"echo-functions": []map[string]any{{"function": "m/p.F", "reason": "r"}}},
			&engine.ConfigError{Setting: engine.SettingEchoFunctions, Problem: engine.ProblemServesNoRule}),
		Entry("an echo function with no reason",
			map[string]any{
				"api": []string{"./..."}, "enable": []string{"inert-field"},
				"echo-functions": []map[string]any{{"function": "m/p.F"}},
			},
			&engine.ConfigError{Setting: engine.SettingEchoFunctions, Problem: engine.ProblemUnexplained, Value: "m/p.F"}),
		Entry("an echo-functions entry naming no function",
			map[string]any{
				"api": []string{"./..."}, "enable": []string{"inert-field"},
				"echo-functions": []map[string]any{{"reason": "r"}},
			},
			&engine.ConfigError{Setting: engine.SettingEchoFunctions, Problem: engine.ProblemNamesNoFunction}),
		Entry("an empty echo-functions list",
			map[string]any{"api": []string{"./..."}, "enable": []string{"inert-field"}, "echo-functions": []any{}},
			&engine.ConfigError{Setting: engine.SettingEchoFunctions, Problem: engine.ProblemEmpty}),
	)

	It("rejects a rule selection that means nothing, naming the key", func() {
		_, err := deadcode.New(map[string]any{"disable": []string{"inert-field"}})

		Expect(err).To(BeAssignableToTypeOf(&engine.ConfigError{}))
		Expect(err).To(HaveField("Problem", engine.ProblemRuleSelection))
		Expect(errors.Unwrap(err)).To(MatchError(
			&ruleset.Error{Key: ruleset.KeyDisable, Problem: ruleset.ProblemNotEnabled, Value: "inert-field"}))
	})

	DescribeTable("hands the engine the rules beside the verdicts",
		func(settings map[string]any, expected []engine.Rule) {
			Expect(engineConfig(settings).Rules).To(Equal(expected))
		},
		Entry("none by default", map[string]any{}, []engine.Rule(nil)),
		Entry("inert-field enabled", map[string]any{"api": []string{"./..."}, "enable": []string{"inert-field"}},
			[]engine.Rule{engine.RuleInertField}),
		Entry("every rule", map[string]any{"api": []string{"./..."}, "default": "all"},
			[]engine.Rule{engine.RuleInertField}),
	)

	It("hands the engine the echo functions", func() {
		Expect(engineConfig(map[string]any{
			"api": []string{"./..."}, "enable": []string{"inert-field"},
			"echo-functions": []map[string]any{{"function": "m/p.(*Spec).Fingerprint", "reason": "an identity"}},
		}).EchoFunctions).To(Equal([]string{"m/p.(*Spec).Fingerprint"}))
	})

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

	It("hands the engine the enum-names credit", func() {
		Expect(engineConfig(map[string]any{"enum-names": true}).EnumNames).To(BeTrue())
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
