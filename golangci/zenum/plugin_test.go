package zenum_test

import (
	"errors"

	"github.com/golangci/plugin-module-register/register"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/ruleset"
	"github.com/zcayou/go-tools/golangci/zenum"
	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("Plugin", func() {
	DescribeTable("hands the engine each accepted setting",
		func(settings any, expected engine.Config) {
			Expect(newPlugin(settings).EngineConfig()).To(Equal(expected))
		},
		Entry("absent", nil, engine.Config{}),
		Entry("empty", map[string]any{}, engine.Config{}),
		Entry("empty lists", map[string]any{
			"open-enums": []any{}, "zero-defaults": []any{}, "related-enums": []any{}, "ordered-enums": []any{},
		}, engine.Config{}),
		Entry("open enums", map[string]any{
			"open-enums": []any{map[string]any{"type": "example.com/wire.ID", "reason": "IDs come from the wire"}},
		}, engine.Config{OpenEnums: []string{"example.com/wire.ID"}}),
		Entry("zero defaults, with a dotted import path", map[string]any{
			"zero-defaults": []any{map[string]any{"type": "example.com/v2.x/kinds.Policy", "reason": "documented"}},
		}, engine.Config{ZeroDefaults: []string{"example.com/v2.x/kinds.Policy"}}),
		Entry("related enums", map[string]any{
			"related-enums": []any{map[string]any{
				"first": "example.com/a.Status", "second": "example.com/b.State", "reason": "different things",
			}},
		}, engine.Config{RelatedEnums: []engine.Pair{{First: "example.com/a.Status", Second: "example.com/b.State"}}}),
		Entry("ordered enums", map[string]any{
			"ordered-enums": []any{map[string]any{"type": "example.com/log.Level", "reason": "severity rises"}},
		}, engine.Config{OrderedEnums: []string{"example.com/log.Level"}}),
		Entry("a rule disabled", map[string]any{"disable": []string{"restated", "numeric-verb"}},
			engine.Config{Disabled: []engine.Rule{engine.RuleRestated, engine.RuleNumericVerb}}),
	)

	It("rejects a rule selection that means nothing, naming the key", func() {
		_, err := zenum.New(map[string]any{"enable": []string{"forged"}})

		Expect(err).To(BeAssignableToTypeOf(&zenum.ConfigError{}))
		Expect(err).To(HaveField("Problem", zenum.ProblemRuleSelection))
		Expect(errors.Unwrap(err)).To(MatchError(
			&ruleset.Error{Key: ruleset.KeyEnable, Problem: ruleset.ProblemUnknownRule, Value: "forged"}))
	})

	DescribeTable("rejects a setting whose every rule the selection leaves off",
		func(settings map[string]any, setting engine.Setting) {
			_, err := zenum.New(settings)

			Expect(err).To(MatchError(&zenum.ConfigError{Problem: zenum.ProblemServesNoRule, Setting: setting}))
		},
		Entry("zero-defaults without zero-member", map[string]any{
			"disable":       []string{"zero-member"},
			"zero-defaults": []any{map[string]any{"type": "example.com/a.A", "reason": "documented"}},
		}, engine.SettingZeroDefaults),
		Entry("related-enums without restated", map[string]any{
			"disable": []string{"restated"},
			"related-enums": []any{map[string]any{
				"first": "example.com/a.A", "second": "example.com/b.B", "reason": "different things",
			}},
		}, engine.SettingRelatedEnums),
		Entry("ordered-enums without ordered-comparison", map[string]any{
			"disable":       []string{"ordered-comparison"},
			"ordered-enums": []any{map[string]any{"type": "example.com/log.Level", "reason": "severity rises"}},
		}, engine.SettingOrderedEnums),
	)

	It("loads the module under the configured build tags", func() {
		Expect(newPlugin(map[string]any{"build-tags": []string{"integration"}}).BuildTags()).To(
			Equal([]string{"integration"}))
	})

	DescribeTable("rejected entries",
		func(settings any, expected *zenum.ConfigError) {
			_, err := zenum.New(settings)

			Expect(err).To(MatchError(expected))
		},
		Entry("an open enum with no reason",
			map[string]any{"open-enums": []any{map[string]any{"type": "example.com/wire.ID"}}},
			&zenum.ConfigError{
				Problem: zenum.ProblemUnexplainedException, Setting: engine.SettingOpenEnums,
			}),
		Entry("a zero default whose reason is blank",
			map[string]any{"zero-defaults": []any{
				map[string]any{"type": "example.com/a.A", "reason": "documented"},
				map[string]any{"type": "example.com/a.B", "reason": "  "},
			}},
			&zenum.ConfigError{Problem: zenum.ProblemUnexplainedException, Setting: engine.SettingZeroDefaults, Index: 1}),
		Entry("a type with no import path",
			map[string]any{"open-enums": []any{map[string]any{"type": "ID", "reason": "wire"}}},
			&zenum.ConfigError{
				Problem: zenum.ProblemUnqualifiedType, Setting: engine.SettingOpenEnums, Type: "ID",
			}),
		Entry("a type that is only an import path",
			map[string]any{"zero-defaults": []any{map[string]any{"type": "example.com/kinds", "reason": "documented"}}},
			&zenum.ConfigError{
				Problem: zenum.ProblemUnqualifiedType, Setting: engine.SettingZeroDefaults, Type: "example.com/kinds",
			}),
		Entry("a related pair missing its second type",
			map[string]any{"related-enums": []any{
				map[string]any{"first": "example.com/a.Status", "reason": "different things"},
			}},
			&zenum.ConfigError{Problem: zenum.ProblemUnqualifiedType, Setting: engine.SettingRelatedEnums}),
		Entry("an ordered enum with no reason",
			map[string]any{"ordered-enums": []any{map[string]any{"type": "example.com/log.Level"}}},
			&zenum.ConfigError{
				Problem: zenum.ProblemUnexplainedException, Setting: engine.SettingOrderedEnums,
			}),
	)

	DescribeTable("rejects a block that does not decode",
		func(settings any) {
			_, err := zenum.New(settings)

			Expect(err).To(BeAssignableToTypeOf(&zenum.ConfigError{}))
			Expect(err).To(HaveField("Problem", zenum.ProblemUndecodableSettings))
		},
		Entry("an unknown key, which is a typo rather than a no-op", map[string]any{"open-enum": []any{}}),
		Entry("an unknown key in an entry", map[string]any{
			"zero-defaults": []any{map[string]any{"type": "example.com/a.A", "reason": "documented", "why": "x"}},
		}),
		Entry("a list where an entry list belongs", map[string]any{"open-enums": []string{"example.com/a.A"}}),
	)

	It("builds without touching the filesystem", func() {
		// golangci-lint builds every configured plugin whether the run enables
		// it or not, and a failure here aborts configuration loading for unrelated
		// commands too.
		chdir(anchorlessDir())

		Expect(newPlugin(map[string]any{
			"open-enums": []any{map[string]any{"type": "example.com/nowhere.ID", "reason": "checked at run time"}},
		})).NotTo(BeNil())
	})

	It("asks for syntax-only loading, because the engine reads its own typed load", func() {
		Expect(newPlugin(nil).GetLoadMode()).To(Equal(register.LoadModeSyntax))
	})
})
