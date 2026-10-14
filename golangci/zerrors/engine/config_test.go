package engine_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("Configuration the module cannot satisfy", func() {
	DescribeTable("fails the run",
		func(cfg engine.Config, expected *engine.ConfigError) {
			_, err := run("judged", cfg)

			Expect(err).To(MatchError(expected))
		},
		Entry("a trusted path the module never imports",
			engine.Config{Trusted: []string{"example.com/nowhere/..."}},
			&engine.ConfigError{
				Setting: engine.SettingTrusted, Problem: engine.ProblemMatchesNothing, Value: "example.com/nowhere/...",
			}),
		Entry("a trusted path naming a module package, which is never foreign",
			engine.Config{Trusted: []string{"example.com/judged"}},
			&engine.ConfigError{
				Setting: engine.SettingTrusted, Problem: engine.ProblemMatchesNothing, Value: "example.com/judged",
			}),
		Entry("a protocol naming no method of the interface",
			engine.Config{Protocols: []string{"example.com/framework.Acquirer.Release"}},
			&engine.ConfigError{
				Setting: engine.SettingProtocols, Problem: engine.ProblemUnresolved,
				Value: "example.com/framework.Acquirer.Release",
			}),
		Entry("a protocol naming a method of no interface",
			engine.Config{Protocols: []string{"example.com/framework.Error.Error"}},
			&engine.ConfigError{
				Setting: engine.SettingProtocols, Problem: engine.ProblemUnresolved,
				Value: "example.com/framework.Error.Error",
			}),
		Entry("a protocol in a package the module never reaches",
			engine.Config{Protocols: []string{"example.com/nowhere.Interface.Method"}},
			&engine.ConfigError{
				Setting: engine.SettingProtocols, Problem: engine.ProblemUnresolved,
				Value: "example.com/nowhere.Interface.Method",
			}),
		Entry("a protocol no module type implements",
			engine.Config{Protocols: []string{"example.com/framework.Source.Next"}},
			&engine.ConfigError{
				Setting: engine.SettingProtocols, Problem: engine.ProblemExemptsNothing,
				Value: "example.com/framework.Source.Next",
			}),
		Entry("a base naming no type",
			engine.Config{Base: "example.com/framework.Nope"},
			&engine.ConfigError{
				Setting: engine.SettingBase, Problem: engine.ProblemUnresolved, Value: "example.com/framework.Nope",
			}),
		Entry("a base naming a type that is neither a struct nor an interface",
			engine.Config{Base: "example.com/framework.Code"},
			&engine.ConfigError{
				Setting: engine.SettingBase, Problem: engine.ProblemWrongKind, Value: "example.com/framework.Code",
			}),
	)

	It("fails on a protocol whose judged implementations an untrusted interface already exempts", func() {
		// Lane is an exported API type, so its Acquire is judged until the module
		// hands it to the untrusted framework's Acquirer.
		_, err := run("exempted", engine.Config{Protocols: []string{"example.com/framework.Acquirer.Acquire"}})

		Expect(err).To(MatchError(&engine.ConfigError{
			Setting: engine.SettingProtocols, Problem: engine.ProblemExemptsNothing,
			Value: "example.com/framework.Acquirer.Acquire",
		}))
	})

	It("accepts the same protocol once trusting the framework judges what it exempts", func() {
		found, err := run("exempted", engine.Config{
			Trusted:   []string{"example.com/framework"},
			Protocols: []string{"example.com/framework.Acquirer.Acquire"},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeEmpty())
	})

	Describe("an entry naming a package the module imports only transitively", func() {
		It("trusts it", func() {
			_, err := run("indirect", engine.Config{Trusted: []string{"example.com/inner"}})

			Expect(err).NotTo(HaveOccurred())
		})

		It("exempts the implementations of its interface's method", func() {
			Expect(analyze("indirect", engine.Config{Protocols: []string{"example.com/inner.Acquirer.Acquire"}},
				engine.RuleUnstructuredReturn)).To(BeEmpty())
		})

		It("resolves a base declared in it, which no export data the load read refers to", func() {
			Expect(analyze("indirect", engine.Config{Base: "example.com/inner.Classified"}, engine.RuleMissingBase)).
				To(ConsistOf("indirect.go:24: missing-base: indirect.Plain does not implement inner.Classified"))
		})
	})

	It("fails on an api pattern matching only packages outside the module, through what resolving it said", func() {
		_, err := run("judged", engine.Config{API: []string{"fmt"}})

		Expect(err).To(BeAssignableToTypeOf(&engine.ConfigError{}))
		Expect(err).To(HaveField("Setting", engine.SettingAPI))
		Expect(err).To(HaveField("Problem", engine.ProblemMatchesNothing))
		Expect(err).To(HaveField("Value", "fmt"))
		pattern, ok := errors.AsType[*wholemodule.PatternError](err)
		Expect(ok).To(BeTrue())
		Expect(pattern.Foreign).To(ConsistOf("fmt"))
	})

	DescribeTable("fails on an api pattern matching no API package",
		func(pattern string) {
			_, err := run("judged", engine.Config{API: []string{pattern}})

			Expect(err).To(BeAssignableToTypeOf(&engine.ConfigError{}))
			Expect(err).To(HaveField("Setting", engine.SettingAPI))
			Expect(err).To(HaveField("Problem", engine.ProblemMatchesNothing))
			Expect(err).To(HaveField("Value", pattern))
			Expect(errors.Unwrap(err)).To(BeAssignableToTypeOf(&wholemodule.PatternError{}))
		},
		Entry("one matching nothing", "./nothing/..."),
		Entry("one matching only internal packages", "./internal/..."),
	)
})
