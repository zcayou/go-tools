package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("forged conversions", func() {
	// The fixture orders a member against a constant, which ordered-comparison
	// judges.
	unordered := engine.Config{Disabled: []engine.Rule{engine.RuleOrderedComparison}}

	// A member's value, zero, a value of the enum already, and any conversion
	// in the declaring package are clean.
	It("reports a computed value, and a constant no member holds, converted outside the declaring package", func() {
		Expect(analyzeFixture("conversions", unordered)).To(Equal([]string{
			"conversions.go:7:3: forged-conversion: example.com/conversions/kinds.Kind",
			"conversions.go:8:3: forged-conversion: example.com/conversions/kinds.Kind",
			// Untyped constants the source gives the enum's type without writing
			// a conversion: a declared constant and variable, an assignment,
			// an argument, arithmetic on a member, and an untyped named constant.
			// A typed constant is judged where it is declared, and a constant compared
			// or looked up with makes no value.
			"implicit.go:7:28: forged-conversion: example.com/conversions/kinds.Kind",
			"implicit.go:11:26: forged-conversion: example.com/conversions/kinds.Kind",
			"implicit.go:14:6: forged-conversion: example.com/conversions/kinds.Kind",
			"implicit.go:18:14: forged-conversion: example.com/conversions/kinds.Kind",
			"implicit.go:19:3: forged-conversion: example.com/conversions/kinds.Kind",
			"implicit.go:20:14: forged-conversion: example.com/conversions/kinds.Kind",
			// A conversion to a type parameter, instantiated with the enum outside its
			// package. The same written in the declaring package is clean.
			"implicit.go:30:37: forged-conversion: example.com/conversions/kinds.Kind",
		}))
	})

	It("says what was converted", func() {
		Expect(messages("conversions", unordered)).To(Equal(map[string]string{
			"conversions.go:7:3": "forged-conversion: converts a computed value to kinds.Kind outside its package; " +
				"only kinds's package may make a member from one",
			"conversions.go:8:3": "forged-conversion: converts 99, which no member holds, to kinds.Kind",
			"implicit.go:7:28":   "forged-conversion: converts 99, which no member holds, to kinds.Kind",
			"implicit.go:11:26":  "forged-conversion: converts 42, which no member holds, to kinds.Kind",
			"implicit.go:14:6":   "forged-conversion: converts 7, which no member holds, to kinds.Kind",
			"implicit.go:18:14":  "forged-conversion: converts 8, which no member holds, to kinds.Kind",
			"implicit.go:19:3":   "forged-conversion: converts 7, which no member holds, to kinds.Kind",
			"implicit.go:20:14":  "forged-conversion: converts 6, which no member holds, to kinds.Kind",
			"implicit.go:30:37": "forged-conversion: converts a computed value to kinds.Kind outside its package; " +
				"only kinds's package may make a member from one",
		}))
	})
})

var _ = Describe("laundered conversions", func() {
	It("reports an ordinal enum converted to another type outside its package", func() {
		Expect(analyzeFixture("laundered", engine.Config{})).To(Equal([]string{
			"kinds/kinds.go:44:6: string-enum: example.com/laundered/kinds.Name",
			// An index, an argument, a member's ordinal, and a named type that is no
			// enum. A conversion into another enum is forged-conversion's; one into
			// the enum itself or an interface, one handed straight to a sync/atomic
			// integer's write, a string enum's, and the declaring package's are clean.
			"laundered.go:20:10: laundered-conversion: example.com/laundered/kinds.Kind",
			"laundered.go:21:16: laundered-conversion: example.com/laundered/kinds.Kind",
			"laundered.go:22:3: laundered-conversion: example.com/laundered/kinds.Kind",
			"laundered.go:23:3: laundered-conversion: example.com/laundered/kinds.Kind",
			"laundered.go:24:3: forged-conversion: example.com/laundered/kinds.Other",
			// Conversions to and from a type parameter, where an instantiation
			// makes them conversions out of the enum.
			"laundered.go:36:45: laundered-conversion: example.com/laundered/kinds.Kind",
			"laundered.go:38:39: laundered-conversion: example.com/laundered/kinds.Kind",
		}))
	})

	It("says what the conversion drops", func() {
		Expect(messages("laundered", engine.Config{})).To(HaveKeyWithValue("laundered.go:23:3",
			"laundered-conversion: converts kinds.Kind to kinds.Code, which keeps its ordinal and drops its type; "+
				"use its String, or convert in example.com/laundered/kinds"))
	})
})
