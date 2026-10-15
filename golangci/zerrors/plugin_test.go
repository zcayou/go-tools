package zerrors_test

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/golangci/plugin-module-register/register"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/ruleset"
	"github.com/zcayou/go-tools/golangci/zerrors"
	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("Plugin", func() {
	DescribeTable("accepted settings",
		func(settings any) {
			plugin, err := zerrors.New(settings)

			Expect(err).NotTo(HaveOccurred())
			Expect(plugin).NotTo(BeNil())
		},
		Entry("absent", nil),
		Entry("empty", map[string]any{}),
		Entry("api", map[string]any{"api": []string{".", "./pkg/..."}}),
		Entry("trusted", map[string]any{"trusted": []string{"github.com/zcayou/gravity/...", "example.com/x"}}),
		Entry("protocols", map[string]any{"protocols": []map[string]any{
			{"method": "example.com/lane.Acquirer.Acquire", "reason": "the lane classifies what it acquires"},
		}}),
		Entry("sentinels off", map[string]any{"sentinels": false}),
		Entry("a base", map[string]any{"base": "github.com/zcayou/gravity/contract.Classification"}),
		Entry("a base scoped to packages", map[string]any{
			"base": "github.com/zcayou/gravity/contract.Classification", "base-packages": []string{"./pkg/..."},
		}),
		Entry("build tags", map[string]any{"build-tags": []string{"integration"}}),
		Entry("an empty trusted list, which trusts nothing", map[string]any{"trusted": []string{}}),
	)

	DescribeTable("rejected settings",
		func(settings any, expected *engine.ConfigError) {
			_, err := zerrors.New(settings)

			Expect(err).To(MatchError(expected))
		},
		Entry("an empty api list, which means the packages in it, of which there are none",
			map[string]any{"api": []string{}},
			&engine.ConfigError{Setting: engine.SettingAPI, Problem: engine.ProblemEmpty}),
		Entry("an empty trusted entry",
			map[string]any{"trusted": []string{""}},
			&engine.ConfigError{Setting: engine.SettingTrusted, Problem: engine.ProblemMalformed}),
		Entry("a trusted wildcard inside a path",
			map[string]any{"trusted": []string{"example.com/.../x"}},
			&engine.ConfigError{
				Setting: engine.SettingTrusted, Problem: engine.ProblemMalformed, Value: "example.com/.../x",
			}),
		Entry("a relative trusted path",
			map[string]any{"trusted": []string{"./pkg"}},
			&engine.ConfigError{Setting: engine.SettingTrusted, Problem: engine.ProblemMalformed, Value: "./pkg"}),
		Entry("a protocol naming no method",
			map[string]any{"protocols": []map[string]any{{"method": "example.com/lane.Acquirer", "reason": "r"}}},
			&engine.ConfigError{
				Setting: engine.SettingProtocols, Problem: engine.ProblemMalformed, Value: "example.com/lane.Acquirer",
			}),
		Entry("a protocol giving no reason",
			map[string]any{"protocols": []map[string]any{{"method": "example.com/lane.Acquirer.Acquire", "reason": " "}}},
			&engine.ConfigError{
				Setting: engine.SettingProtocols, Problem: engine.ProblemNoReason,
				Value: "example.com/lane.Acquirer.Acquire",
			}),
		Entry("a base naming no type",
			map[string]any{"base": "example.com/contract"},
			&engine.ConfigError{
				Setting: engine.SettingBase, Problem: engine.ProblemMalformed, Value: "example.com/contract",
			}),
		Entry("an empty base",
			map[string]any{"base": ""},
			&engine.ConfigError{Setting: engine.SettingBase, Problem: engine.ProblemMalformed}),
		Entry("an empty base-packages",
			map[string]any{"base": "example.com/contract.Classification", "base-packages": []string{}},
			&engine.ConfigError{Setting: engine.SettingBasePackages, Problem: engine.ProblemEmpty}),
		Entry("base-packages without base, which scopes a rule that is off",
			map[string]any{"base-packages": []string{"./pkg/..."}},
			&engine.ConfigError{Setting: engine.SettingBasePackages, Problem: engine.ProblemWithoutBase}),
	)

	DescribeTable("settings that do not decode",
		func(settings any) {
			_, err := zerrors.New(settings)

			Expect(err).To(BeAssignableToTypeOf(&engine.ConfigError{}))
			Expect(err).To(HaveField("Problem", engine.ProblemUndecodable))
			Expect(errors.Unwrap(err)).To(MatchError(HavePrefix("decoding settings: ")))
		},
		Entry("an unknown key, which is a typo rather than a no-op", map[string]any{"sentinel": false}),
		Entry("an unknown key in a protocol", map[string]any{"protocols": []map[string]any{
			{"method": "example.com/lane.Acquirer.Acquire", "reason": "r", "why": "r"},
		}}),
		Entry("a value of the wrong type", map[string]any{"api": "./..."}),
	)

	It("hands the engine the defaults when nothing is set", func() {
		Expect(engineConfig(nil)).To(Equal(engine.Config{Sentinels: true, Protocols: []string{}}))
	})

	It("hands the engine every setting", func() {
		config := engineConfig(map[string]any{
			"api":           []string{"./pkg/..."},
			"trusted":       []string{"example.com/framework/..."},
			"protocols":     []map[string]any{{"method": "example.com/lane.Acquirer.Acquire", "reason": "r"}},
			"sentinels":     false,
			"base":          "example.com/contract.Classification",
			"base-packages": []string{"./pkg/...", "./internal/..."},
			"build-tags":    []string{"integration"},
		})

		Expect(config).To(Equal(engine.Config{
			API:          []string{"./pkg/..."},
			Trusted:      []string{"example.com/framework/..."},
			Protocols:    []string{"example.com/lane.Acquirer.Acquire"},
			Base:         "example.com/contract.Classification",
			BasePackages: []string{"./pkg/...", "./internal/..."},
			BuildTags:    []string{"integration"},
		}))
	})

	It("hands the engine the rules the selection leaves off", func() {
		config := engineConfig(map[string]any{"disable": []string{"text-decision", "missing-base"}})

		Expect(config.Disabled).To(Equal([]engine.Rule{engine.RuleTextDecision, engine.RuleMissingBase}))
	})

	It("rejects a rule selection that means nothing, naming the key", func() {
		_, err := zerrors.New(map[string]any{"default": "none"})

		Expect(err).To(BeAssignableToTypeOf(&engine.ConfigError{}))
		Expect(err).To(HaveField("Problem", engine.ProblemRuleSelection))
		Expect(errors.Unwrap(err)).To(MatchError(
			&ruleset.Error{Key: ruleset.KeyDefault, Problem: ruleset.ProblemNoRuleEnabled}))
	})

	DescribeTable("rejects a setting whose every rule the selection leaves off",
		func(settings map[string]any, setting engine.Setting) {
			_, err := zerrors.New(settings)

			Expect(err).To(MatchError(&engine.ConfigError{Setting: setting, Problem: engine.ProblemServesNoRule}))
		},
		Entry("api without unstructured-return, literal-field or rendered-field", map[string]any{
			"disable": []string{"unstructured-return", "literal-field", "rendered-field"}, "api": []string{"./..."},
		},
			engine.SettingAPI),
		Entry("trusted without unstructured-return",
			map[string]any{"disable": []string{"unstructured-return"}, "trusted": []string{"example.com/x"}},
			engine.SettingTrusted),
		Entry("protocols without unstructured-return",
			map[string]any{"disable": []string{"unstructured-return"}, "protocols": []map[string]any{
				{"method": "example.com/lane.Acquirer.Acquire", "reason": "r"},
			}},
			engine.SettingProtocols),
		Entry("sentinels without unstructured-return",
			map[string]any{"disable": []string{"unstructured-return"}, "sentinels": false}, engine.SettingSentinels),
		Entry("base without unstructured-return or missing-base",
			map[string]any{"disable": []string{"unstructured-return", "missing-base"}, "base": "example.com/c.B"},
			engine.SettingBase),
		Entry("base-packages without missing-base", map[string]any{
			"disable": []string{"missing-base"}, "base": "example.com/c.B", "base-packages": []string{"./..."},
		}, engine.SettingBasePackages),
	)

	It("builds without touching the filesystem", func() {
		// golangci-lint builds every configured plugin whether the run enables
		// it or not, and a failure here aborts configuration loading for unrelated
		// commands too.
		chdir(anchorlessDir())

		plugin, err := zerrors.New(map[string]any{"base": "example.com/nowhere.Base"})

		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).NotTo(BeNil())
	})

	It("asks for syntax-only loading, because the engine does its own typed load", func() {
		plugin, err := zerrors.New(nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(plugin.GetLoadMode()).To(Equal(register.LoadModeSyntax))
	})
})

// engineConfig builds the plugin from settings and returns the configuration
// it hands the engine.
func engineConfig(settings map[string]any) engine.Config {
	GinkgoHelper()

	plugin, err := zerrors.New(settings)
	Expect(err).NotTo(HaveOccurred())
	built, ok := plugin.(*zerrors.Plugin)
	Expect(ok).To(BeTrue())
	return built.EngineConfig()
}

// anchorlessDir returns a directory with no go.mod at or above it, which
// is what a repository golangci-lint cannot anchor looks like from the plugin.
func anchorlessDir() string {
	GinkgoHelper()

	dir, err := os.MkdirTemp("", "zerrors-anchorless")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { Expect(os.RemoveAll(dir)).To(Succeed()) })
	resolved, err := filepath.EvalSymlinks(dir)
	Expect(err).NotTo(HaveOccurred())
	Expect(filepath.Join(resolved, "go.mod")).NotTo(BeAnExistingFile())
	return resolved
}
