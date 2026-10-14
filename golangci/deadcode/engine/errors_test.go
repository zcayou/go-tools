package engine_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

var _ = Describe("ConfigError", func() {
	DescribeTable("renders the setting at fault and what is wrong with it",
		func(err *engine.ConfigError, message string) {
			Expect(err).To(MatchError(message))
		},
		Entry("an empty list",
			&engine.ConfigError{Setting: engine.SettingTestFacing, Problem: engine.ProblemEmpty},
			"test-facing is empty: omit it to declare no test-facing packages"),
		Entry("an unknown exemption",
			&engine.ConfigError{Setting: engine.SettingAPIExempt, Problem: engine.ProblemUnknownValue, Value: "nonsense"},
			`unknown api-exempt "nonsense": want one of func, method, type, const, var, interface-method`),
		Entry("a setting without api",
			&engine.ConfigError{Setting: engine.SettingAPIGenerics, Problem: engine.ProblemRequiresAPI},
			"api-generics requires api"),
		Entry("a setting without tests",
			&engine.ConfigError{Setting: engine.SettingAPITestConsumers, Problem: engine.ProblemRequiresTests},
			"api-test-consumers requires tests: the masked view is all it speaks to"),
		Entry("a pattern matching no package",
			&engine.ConfigError{Setting: engine.SettingAPI, Problem: engine.ProblemMatchesNothing, Value: "./x/..."},
			"api pattern ./x/... matched no packages"),
		Entry("a roots pattern matching no file",
			&engine.ConfigError{Setting: engine.SettingRoots, Problem: engine.ProblemMatchesNothing, Value: "gen/*.go"},
			`roots pattern "gen/*.go" matched no files`),
		Entry("a package declared twice over",
			&engine.ConfigError{
				Setting: engine.SettingTestFacing, Problem: engine.ProblemOverlapsAPI, Packages: []string{"a", "b"},
			},
			"declared both api and test-facing: a, b"),
		Entry("the reserved directory",
			&engine.ConfigError{Setting: engine.SettingRoots, Problem: engine.ProblemReservedDirectory, Value: "/m"},
			".deadcode-roots exists in /m: the directory is reserved for synthesized root programs"),
		Entry("a root program of another package",
			&engine.ConfigError{
				Setting: engine.SettingRoots, Problem: engine.ProblemNotMain, Value: "lib/lib.go", Package: "lib",
			},
			"root program lib/lib.go declares package lib, not main"),
		Entry("a root program with no main",
			&engine.ConfigError{Setting: engine.SettingRoots, Problem: engine.ProblemNoMain, Value: "p/prog.go"},
			"root program p/prog.go declares no func main"),
		Entry("settings that do not decode, in the decoder's words",
			engine.Undecodable(errors.New(`decoding settings: json: unknown field "pattern"`)),
			`decoding settings: json: unknown field "pattern"`),
		Entry("settings that do not decode, with nothing said",
			&engine.ConfigError{Problem: engine.ProblemUndecodable},
			"settings do not decode"),
		Entry("an unresolvable pattern with nothing said",
			&engine.ConfigError{Setting: engine.SettingAPI, Problem: engine.ProblemUnresolvable, Value: "./x"},
			"resolving api pattern ./x"),
		Entry("an unreadable root program with nothing said",
			&engine.ConfigError{Setting: engine.SettingRoots, Problem: engine.ProblemUnreadable, Value: "p/prog.go"},
			"reading root program p/prog.go"),
		Entry("an empty setting that is not a list",
			&engine.ConfigError{Setting: engine.SettingAPI, Problem: engine.ProblemEmpty},
			"api is empty"),
		Entry("an unknown value of a setting with no closed set",
			&engine.ConfigError{Setting: engine.SettingPatterns, Problem: engine.ProblemUnknownValue, Value: "x"},
			`unknown patterns "x"`),
		Entry("a problem outside the enum",
			&engine.ConfigError{},
			"Setting(0): Problem(0)"),
	)

	It("renders a pattern the loader could not resolve with what it said", func(ctx SpecContext) {
		_, err := engine.Analyze(ctx, engine.Config{
			Dir:      fixtureDir("control"),
			Patterns: []string{"./..."},
			API:      []string{"./nope"},
		})

		Expect(err).To(MatchError(HavePrefix("resolving api pattern ./nope: ")))
		Expect(errors.Unwrap(err)).To(MatchError(ContainSubstring("nope")))
	})

	It("names each setting and problem by its token", func() {
		Expect([]string{
			engine.SettingPatterns.String(), engine.SettingAPI.String(), engine.SettingAPIExempt.String(),
			engine.SettingAPIGenerics.String(), engine.SettingAPITestConsumers.String(),
			engine.SettingTestFacing.String(), engine.SettingRoots.String(),
		}).To(Equal([]string{
			"patterns", "api", "api-exempt", "api-generics", "api-test-consumers", "test-facing", "roots",
		}))
		Expect([]string{
			engine.ProblemUndecodable.String(), engine.ProblemEmpty.String(), engine.ProblemUnknownValue.String(),
			engine.ProblemRequiresAPI.String(), engine.ProblemRequiresTests.String(),
			engine.ProblemUnresolvable.String(), engine.ProblemMatchesNothing.String(),
			engine.ProblemOverlapsAPI.String(), engine.ProblemReservedDirectory.String(),
			engine.ProblemUnreadable.String(), engine.ProblemNotMain.String(), engine.ProblemNoMain.String(),
		}).To(Equal([]string{
			"undecodable", "empty", "unknown-value", "requires-api", "requires-tests", "unresolvable",
			"matches-nothing", "overlaps-api", "reserved-directory", "unreadable", "not-main", "no-main",
		}))
	})
})

var _ = Describe("RunError", func() {
	It("renders the stage the analysis stopped at and why", func(ctx SpecContext) {
		canceled, cancel := context.WithCancel(ctx)
		cancel()

		_, err := engine.Analyze(canceled, engine.Config{Dir: fixtureDir("control"), Patterns: []string{"./..."}})

		Expect(err).To(MatchError("loading packages: context canceled"))
	})

	It("renders a stage stopped with nothing said, and a stage outside the enum", func() {
		Expect(&engine.RunError{Stage: engine.StageBuildingSSA}).To(MatchError("building ssa: stopped"))
		Expect(&engine.RunError{}).To(MatchError("Stage(0): stopped"))
	})

	It("names each stage", func() {
		Expect([]string{
			engine.StageLoadingPackages.String(), engine.StageBuildingSSA.String(),
			engine.StageScanningBinds.String(), engine.StageAnalyzingReachability.String(),
		}).To(Equal([]string{
			"loading packages", "building ssa", "scanning interface binds", "analyzing reachability",
		}))
	})
})
