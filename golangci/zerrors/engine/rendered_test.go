package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("rendered-field", func() {
	It("reports a field only ever holding the String of one type a caller could carry", func() {
		// NotFoundError holds a key's rendering, GenericError the one instantiation
		// a generic body renders, and ThroughError a rendering through a local
		// and an unexported function. A field also holding a constant, one holding
		// two types' renderings or two instantiations', and one holding the String
		// of a type that is not comparable, an enum, internal, unexported, or called
		// through an interface are not reported.
		Expect(analyze("rendered", engine.Config{}, engine.RuleRenderedField)).To(ConsistOf(
			"rendered.go:11: rendered-field: rendered.NotFoundError.Key holds only the String of "+
				"example.com/rendered/store.Key; carry store.Key",
			"rendered.go:23: rendered-field: rendered.GenericError.Ref holds only the String of "+
				"example.com/rendered/store.Ref[int]; carry store.Ref[int]",
			"rendered.go:31: rendered-field: rendered.ThroughError.Key holds only the String of "+
				"example.com/rendered/store.Key; carry store.Key",
		))
	})
})
