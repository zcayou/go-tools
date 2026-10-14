package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("forged conversions", func() {
	// A member's value, zero, a value of the enum already, and any conversion
	// in the declaring package are clean.
	It("reports a computed value, and a constant no member holds, converted outside the declaring package", func() {
		Expect(analyzeFixture("conversions", engine.Config{})).To(Equal([]string{
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
		Expect(messages("conversions", engine.Config{})).To(Equal(map[string]string{
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
