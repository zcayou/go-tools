package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("literal-field", func() {
	It("reports a field only ever fed constants, however they reach it", func() {
		// Exported is fed through an exported function's parameter, Display
		// by an internal type's String, Dynamic by one call it cannot follow, Typed
		// is a named string type and Never is never stored. Decoded is also written
		// by a decoder handed it, Converted converted from a struct holding
		// a parameter, Fault fed in an uninstantiated generic body and Op through
		// a parameter such a body passes a value: none is reported. Handled is fed
		// from an unexported field of a struct handed out as any, which no code
		// outside the module can write.
		Expect(analyze("literal", engine.Config{}, engine.RuleLiteralField)).To(ConsistOf(
			"literal.go:33: literal-field: literal.Literal.Reason only ever holds constants; give it an enum type",
			"literal.go:37: literal-field: literal.Param.Reason only ever holds constants; give it an enum type",
			HavePrefix("literal.go:41: literal-field: literal.Carried.Reason only ever holds \"x\""),
			"literal.go:45: literal-field: literal.Named.Reason only ever holds constants; give it an enum type",
			HavePrefix("literal.go:49: literal-field: literal.Tags.Tags only ever holds []string{\"a\", \"b\"}"),
			HavePrefix("literal.go:167: literal-field: literal.Handled.Target only ever holds \"container\""),
			"literal.go:207: literal-field: literal.Partial.Reason only ever holds constants; give it an enum type",
		))
	})

	It("says a field that only ever holds one value carries no information", func() {
		// Carried holds one string through a carrier struct, Tags one list,
		// and Handled one string through a driver's field. Partial is also left
		// unset, so it holds its constant or nothing.
		Expect(analyze("literal", engine.Config{}, engine.RuleLiteralField)).To(ContainElements(
			"literal.go:41: literal-field: literal.Carried.Reason only ever holds \"x\", "+
				"so it carries no information; remove it",
			"literal.go:49: literal-field: literal.Tags.Tags only ever holds []string{\"a\", \"b\"}, "+
				"so it carries no information; remove it",
			"literal.go:167: literal-field: literal.Handled.Target only ever holds \"container\", "+
				"so it carries no information; remove it",
		))
	})
})
