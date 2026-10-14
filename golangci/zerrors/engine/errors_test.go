package engine_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("ConfigError", func() {
	DescribeTable("names the setting, the entry and what is wrong with it",
		func(err *engine.ConfigError, text string) {
			Expect(err).To(MatchError(text))
		},
		Entry("an empty list",
			&engine.ConfigError{Setting: engine.SettingAPI, Problem: engine.ProblemEmpty},
			"api is empty: omit it to judge every package as API"),
		Entry("a malformed trusted path",
			&engine.ConfigError{Setting: engine.SettingTrusted, Problem: engine.ProblemMalformed, Value: "./x"},
			`trusted: "./x" is not an import path, optionally ending in /...`),
		Entry("a malformed protocol",
			&engine.ConfigError{Setting: engine.SettingProtocols, Problem: engine.ProblemMalformed, Value: "a.I"},
			`protocols: "a.I" is not written import/path.Interface.Method`),
		Entry("a malformed base",
			&engine.ConfigError{Setting: engine.SettingBase, Problem: engine.ProblemMalformed, Value: "a"},
			`base: "a" is not written import/path.Name`),
		Entry("a protocol giving no reason",
			&engine.ConfigError{Setting: engine.SettingProtocols, Problem: engine.ProblemNoReason, Value: "a.I.M"},
			`protocols: "a.I.M" gives no reason`),
		Entry("a trusted path matching nothing",
			&engine.ConfigError{Setting: engine.SettingTrusted, Problem: engine.ProblemMatchesNothing, Value: "a/b"},
			`trusted: "a/b" matches no package the module imports`),
		Entry("an unresolved protocol",
			&engine.ConfigError{Setting: engine.SettingProtocols, Problem: engine.ProblemUnresolved, Value: "a.I.M"},
			`protocols: "a.I.M" names no interface method`),
		Entry("a protocol exempting nothing",
			&engine.ConfigError{Setting: engine.SettingProtocols, Problem: engine.ProblemExemptsNothing, Value: "a.I.M"},
			`protocols: "a.I.M" exempts nothing: no method it names is otherwise judged`),
		Entry("an unresolved base",
			&engine.ConfigError{Setting: engine.SettingBase, Problem: engine.ProblemUnresolved, Value: "a.B"},
			`base: "a.B" names no type`),
		Entry("a base of the wrong kind",
			&engine.ConfigError{Setting: engine.SettingBase, Problem: engine.ProblemWrongKind, Value: "a.B"},
			`base: "a.B" names neither a struct nor an interface`),
		Entry("an empty base-packages",
			&engine.ConfigError{Setting: engine.SettingBasePackages, Problem: engine.ProblemEmpty},
			"base-packages is empty: omit it to judge every package's error types"),
		Entry("base-packages without base",
			&engine.ConfigError{Setting: engine.SettingBasePackages, Problem: engine.ProblemWithoutBase},
			"base-packages is set without base, so it scopes nothing"),
		Entry("settings that do not decode, through what the decoder said",
			engine.Undecodable(errors.New(`decoding settings: json: unknown field "nope"`)),
			`decoding settings: json: unknown field "nope"`),
		Entry("settings that do not decode, with nothing said",
			engine.Undecodable(nil), "settings do not decode"),
		Entry("an empty list of a setting other than api",
			&engine.ConfigError{Setting: engine.SettingTrusted, Problem: engine.ProblemEmpty},
			"trusted is empty"),
		Entry("a malformed entry of a setting with no form of its own",
			&engine.ConfigError{Setting: engine.SettingAPI, Problem: engine.ProblemMalformed, Value: "x"},
			`api: "x" is malformed`),
		Entry("an api pattern matching nothing, with nothing said",
			&engine.ConfigError{Setting: engine.SettingAPI, Problem: engine.ProblemMatchesNothing, Value: "./x"},
			`api: "./x" matches nothing`),
		Entry("an unresolved entry of a setting naming no declaration",
			&engine.ConfigError{Setting: engine.SettingTrusted, Problem: engine.ProblemUnresolved, Value: "x"},
			`trusted: "x" names nothing`),
		Entry("a wrong kind outside base",
			&engine.ConfigError{Setting: engine.SettingProtocols, Problem: engine.ProblemWrongKind, Value: "x"},
			`protocols: "x" names the wrong kind of declaration`),
		Entry("exempting nothing outside protocols",
			&engine.ConfigError{Setting: engine.SettingTrusted, Problem: engine.ProblemExemptsNothing, Value: "x"},
			`trusted: "x" exempts nothing`),
		Entry("no setting and no problem",
			&engine.ConfigError{}, `Setting(0): "": Problem(0)`),
		Entry("a problem past the known ones",
			&engine.ConfigError{Setting: engine.SettingBase, Problem: engine.Problem(99), Value: "a.B"},
			`base: "a.B": Problem(99)`),
	)

	It("renders a base-packages pattern through what resolving it ran into", func() {
		_, err := run("base", engine.Config{
			Base: "example.com/base/contract.Classification", BasePackages: []string{"./nowhere/..."},
		})

		Expect(err).To(MatchError(HavePrefix("base-packages: resolving pattern ./nowhere/...: ")))
	})

	It("renders an api pattern through what resolving it ran into", func() {
		_, err := run("judged", engine.Config{API: []string{"./internal/..."}})

		Expect(err).To(MatchError("api: pattern ./internal/... matched only main or internal packages, " +
			"which are not API: example.com/judged/internal/hidden"))
	})
})
