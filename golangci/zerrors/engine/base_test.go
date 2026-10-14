package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("missing-base", func() {
	It("reports an error type not embedding a struct base by value", func() {
		// NotFound embeds the base, Chained embeds it through an embedding,
		// classified embeds it unexported, and codes.Invalid sits in a package
		// the base's own imports.
		Expect(analyze("base", engine.Config{Base: "example.com/base/contract.Classification"},
			engine.RuleMissingBase)).To(ConsistOf(
			"store/store.go:12: missing-base: store.Pointer does not embed contract.Classification by value",
			"store/store.go:16: missing-base: store.Missing does not embed contract.Classification by value",
			"store/store.go:22: missing-base: store.lost does not embed contract.Classification by value",
			"internal/wrap/wrap.go:4: missing-base: wrap.Exec does not embed contract.Classification by value",
			"tools/tools.go:4: missing-base: tools.Failed does not embed contract.Classification by value",
		))
	})

	It("judges only the packages base-packages names, internal ones included", func() {
		// tools stays outside the framework, and no pattern names it.
		Expect(analyze("base", engine.Config{
			Base:         "example.com/base/contract.Classification",
			BasePackages: []string{"./store/...", "./internal/..."},
		}, engine.RuleMissingBase)).To(ConsistOf(
			"store/store.go:12: missing-base: store.Pointer does not embed contract.Classification by value",
			"store/store.go:16: missing-base: store.Missing does not embed contract.Classification by value",
			"store/store.go:22: missing-base: store.lost does not embed contract.Classification by value",
			"internal/wrap/wrap.go:4: missing-base: wrap.Exec does not embed contract.Classification by value",
		))
	})

	It("fails on a base-packages pattern matching no package of the module", func() {
		_, err := run("base", engine.Config{
			Base:         "example.com/base/contract.Classification",
			BasePackages: []string{"./store/...", "./nowhere/..."},
		})

		Expect(err).To(BeAssignableToTypeOf(&engine.ConfigError{}))
		Expect(err).To(HaveField("Setting", engine.SettingBasePackages))
		Expect(err).To(HaveField("Problem", engine.ProblemMatchesNothing))
		Expect(err).To(HaveField("Value", "./nowhere/..."))
	})

	It("reports an error type implementing an interface base through neither T nor *T", func() {
		// Generic declares Classify, with a signature other than the base's.
		Expect(analyze("ifacebase", engine.Config{Base: "example.com/framework.Classified"}, engine.RuleMissingBase)).
			To(ConsistOf(
				"ifacebase.go:18: missing-base: ifacebase.Bad does not implement framework.Classified",
				"ifacebase.go:25: missing-base: ifacebase.Generic does not implement framework.Classified",
			))
	})

	It("is off without a base", func() {
		Expect(analyze("base", engine.Config{}, engine.RuleMissingBase)).To(BeEmpty())
	})
})
