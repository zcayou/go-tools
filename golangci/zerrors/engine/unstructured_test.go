package engine_test

import (
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("unstructured-return", func() {
	// Every origin the origins fixture returns, as an untrusted framework
	// and counted sentinels leave them.
	untrusted := []string{
		"origins.go:28: unstructured-return: New can return an unstructured error: errors.New",
		"origins.go:30: unstructured-return: Errorf can return an unstructured error: fmt.Errorf",
		"origins.go:32: unstructured-return: ErrorfWrappingRaw can return an unstructured error: fmt.Errorf",
		"origins.go:34: unstructured-return: JoinRaw can return an unstructured error: errors.Join",
		"origins.go:36: unstructured-return: Unexported can return an unstructured error: " +
			"a value of type origins.notFound",
		"origins.go:38: unstructured-return: ForeignCall can return an unstructured error: os.Remove",
		"origins.go:40: unstructured-return: ForeignInterface can return an unstructured error: context.Context.Err",
		"origins.go:42: unstructured-return: ForeignVariable can return an unstructured error: io.EOF",
		"origins.go:44: unstructured-return: ForeignField can return an unstructured error: framework.Carrier.Err",
		"origins.go:46: unstructured-return: ForeignChannelField can return an unstructured error: " +
			"a receive from framework.Carrier.Errs",
		"origins.go:48: unstructured-return: ForeignChannelCall can return an unstructured error: " +
			"a receive from framework.Results",
		"origins.go:50: unstructured-return: FrameworkCall can return an unstructured error: framework.Do",
		"origins.go:52: unstructured-return: FrameworkVariable can return an unstructured error: framework.ErrClosed",
		"origins.go:54: unstructured-return: FrameworkInterface can return an unstructured error: framework.Source.Next",
		"origins.go:56: unstructured-return: FrameworkType can return an unstructured error: " +
			"a value of type framework.Error",
		"origins.go:103: unstructured-return: Branches can return an unstructured error: io.ErrUnexpectedEOF",
		"origins.go:112: unstructured-return: Deferred can return an unstructured error: fmt.Errorf",
	}

	It("reports each unstructured origin, and no structured or unjudged one", func() {
		// Nameable types, counted sentinels, %w and Join with a structured operand,
		// a judged function's parameter, function values, WalkDir and an errgroup
		// handed callbacks, other receives, loads from slices, maps and pointers,
		// assertions, type parameters and a non-constant format draw nothing.
		Expect(analyze("origins", engine.Config{Sentinels: true}, engine.RuleUnstructuredReturn)).
			To(ConsistOf(untrusted))
	})

	It("counts whatever a trusted package provides as structured", func() {
		trusted := analyze("origins", engine.Config{Sentinels: true, Trusted: []string{"example.com/framework"}},
			engine.RuleUnstructuredReturn)

		Expect(trusted).To(ConsistOf(slices.Concat(untrusted[:8], untrusted[15:])))
	})

	It("counts a sentinel as its initializer while sentinels are off", func() {
		Expect(analyze("origins", engine.Config{}, engine.RuleUnstructuredReturn)).To(ConsistOf(append(untrusted,
			"origins.go:60: unstructured-return: Sentinel can return an unstructured error: "+
				"errors.New in the initializer of ErrGone")))
	})

	It("counts an unexported error type carrying the base as structured", func() {
		Expect(analyze("base", engine.Config{Base: "example.com/base/contract.Classification"},
			engine.RuleUnstructuredReturn)).To(ConsistOf(
			"store/store.go:28: unstructured-return: Lose can return an unstructured error: a value of type store.lost",
		))
	})

	It("names the witness earliest by file and offset, qualified by the package it sits in", func() {
		Expect(analyze("witnesses", engine.Config{API: []string{"."}}, engine.RuleUnstructuredReturn)).To(ConsistOf(
			"witnesses.go:37: unstructured-return: Pick can return an unstructured error: errors.New in a.rawa",
		))
	})

	It("leaves a value an instance's type argument makes unstructured with the caller", func() {
		// Wrap's own body returns a value of its type parameter, which is not
		// judged, so the caller instantiating it with an unnameable type carries
		// the finding.
		Expect(analyze("generics", engine.Config{}, engine.RuleUnstructuredReturn)).To(ContainElement(
			"generics.go:39: unstructured-return: WrapRaw can return an unstructured error: " +
				"a value of type generics.raw in Wrap",
		))
	})

	It("counts an error type an exported alias of an API package names as nameable", func() {
		Expect(analyze("aliases", engine.Config{}, engine.RuleUnstructuredReturn)).NotTo(ContainElement(
			HavePrefix("aliases.go:13:")))
		Expect(analyze("aliases", engine.Config{}, engine.RuleUnstructuredReturn)).To(ContainElement(
			"aliases.go:15: unstructured-return: Hidden can return an unstructured error: a value of type store.Hidden",
		))
	})
})
