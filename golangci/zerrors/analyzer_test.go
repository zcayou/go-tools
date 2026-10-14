package zerrors_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/zerrors"
)

var _ = Describe("Analyzer", func() {
	It("exposes the analyzer golangci-lint should run, alongside the cache defeat", func() {
		plugin, err := zerrors.New(nil)
		Expect(err).NotTo(HaveOccurred())

		analyzers, err := plugin.BuildAnalyzers()

		Expect(err).NotTo(HaveOccurred())
		Expect(analyzers).To(HaveLen(2))
		Expect(analyzers[0].Name).To(Equal(zerrors.Name))
		Expect(analyzers[0].Doc).NotTo(BeEmpty())
		Expect(analyzers[0].FactTypes).To(BeEmpty())
		Expect(analyzers[0].Requires).To(BeEmpty())
		Expect(analyzers[1].Name).To(HavePrefix(zerrors.Name + "_uncacheable_"))
	})

	It("reports each finding in the pass that holds its file, opening with its rule", func() {
		reported, err := runFixture("plain")

		Expect(err).NotTo(HaveOccurred())
		Expect(reported).To(ConsistOf(
			"plain.go:11:2: [unstructured-return] unstructured-return: Open can return an unstructured error: errors.New",
			"plain.go:15:25: [text-decision] text-decision: decides on an error's text; "+
				"branch on its type or fields instead",
		))
	})

	It("fails every pass on a module that does not load", func() {
		dir := fixtureDir("broken")

		_, err := runFixture("broken")

		Expect(err).To(BeAssignableToTypeOf(&zerrors.LoadError{}))
		Expect(err).To(HaveField("Root", dir))
		Expect(errors.Unwrap(err)).To(BeAssignableToTypeOf(&wholemodule.LoadError{}))
	})
})
