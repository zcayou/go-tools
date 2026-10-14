package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("The judged functions", func() {
	const (
		fetch   = "judged.go:18: unstructured-return: fetcher.Fetch can return an unstructured error: errors.New"
		handle  = "judged.go:24: unstructured-return: handler.Handle can return an unstructured error: errors.New"
		acquire = "judged.go:28: unstructured-return: lane.Acquire can return an unstructured error: errors.New"
		gone    = "judged.go:56: unstructured-return: Gone can return an unstructured error: " +
			"errors.New in the initializer of ErrGone"
		hidden = "judged.go:58: unstructured-return: Hidden can return an unstructured error: " +
			"errors.New in the initializer of errHidden"
		produce  = "judged.go:60: unstructured-return: Produce can return an unstructured error: errors.New"
		mixed    = "judged.go:72: unstructured-return: Mixed can return an unstructured error: io.EOF"
		internal = "judged.go:75: unstructured-return: Internal can return an unstructured error: errors.New in hidden.Raw"
	)

	It("judges a method reached through an exported interface, and exempts Unwrap and a foreign contract", func() {
		// The handler and the lane answer to an untrusted framework's interfaces,
		// Buffer.Write to io.Writer's through a blank assertion, and Unwrap hands
		// out a cause: none is judged. Relay and RelayThroughHelper return only
		// what Produce did, which Produce's own return carries, and the internal
		// package is never API.
		Expect(analyze("judged", engine.Config{Sentinels: true}, engine.RuleUnstructuredReturn)).To(ConsistOf(
			fetch, hidden, produce, mixed, internal,
		))
	})

	It("judges the implementations of a trusted interface", func() {
		Expect(analyze("judged", engine.Config{Sentinels: true, Trusted: []string{"example.com/framework"}},
			engine.RuleUnstructuredReturn)).To(ConsistOf(fetch, handle, acquire, hidden, produce, mixed, internal))
	})

	It("exempts the implementations of a protocol", func() {
		Expect(analyze("judged", engine.Config{
			Sentinels: true,
			Trusted:   []string{"example.com/framework/..."},
			Protocols: []string{"example.com/framework.Acquirer.Acquire"},
		}, engine.RuleUnstructuredReturn)).To(ConsistOf(fetch, handle, hidden, produce, mixed, internal))
	})

	It("judges exported sentinels by their initializers while sentinels are off", func() {
		Expect(analyze("judged", engine.Config{}, engine.RuleUnstructuredReturn)).To(ConsistOf(
			fetch, gone, hidden, produce, mixed, internal,
		))
	})

	It("never makes an internal package API, whatever pattern matches it", func() {
		Expect(analyze("judged", engine.Config{Sentinels: true, API: []string{"./..."}},
			engine.RuleUnstructuredReturn)).To(ConsistOf(fetch, hidden, produce, mixed, internal))
	})

	It("judges methods with type parameters of their own, and a generic API interface's implementations", func() {
		// UseBox calls Box.Get, which carries the finding itself.
		Expect(analyze("generics", engine.Config{}, engine.RuleUnstructuredReturn)).To(ConsistOf(
			"generics.go:11: unstructured-return: Box.Get can return an unstructured error: errors.New",
			"generics.go:20: unstructured-return: Set.Map can return an unstructured error: errors.New",
			"generics.go:29: unstructured-return: intGetter.Get can return an unstructured error: errors.New",
			"generics.go:39: unstructured-return: WrapRaw can return an unstructured error: "+
				"a value of type generics.raw in Wrap",
		))
	})

	It("judges the methods of a type an exported alias of an API package names", func() {
		Expect(analyze("aliases", engine.Config{}, engine.RuleUnstructuredReturn)).To(ContainElement(
			"internal/store/store.go:16: unstructured-return: Store.Get can return an unstructured error: errors.New",
		))
	})
})
