package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("numeric verbs", func() {
	// Kind's own String prints a value no member holds under %d, which is not
	// reported.
	It("reports an ordinal enum value under each verb that bypasses its String", func() {
		Expect(analyzeFixture("verbs", engine.Config{})).To(Equal([]string{
			"verbs.go:26:6: string-enum: example.com/verbs.Mode",
			// %d %x %X %o %b %c %U.
			"verbs.go:31:39: numeric-verb: example.com/verbs.Kind",
			"verbs.go:31:42: numeric-verb: example.com/verbs.Kind",
			"verbs.go:31:45: numeric-verb: example.com/verbs.Kind",
			"verbs.go:31:48: numeric-verb: example.com/verbs.Kind",
			"verbs.go:31:51: numeric-verb: example.com/verbs.Kind",
			"verbs.go:31:54: numeric-verb: example.com/verbs.Kind",
			"verbs.go:31:57: numeric-verb: example.com/verbs.Kind",
			// An explicit index, a * width, a * precision on a method, and fmt.Errorf.
			// Operands spread from a slice cannot be read.
			"verbs.go:40:36: numeric-verb: example.com/verbs.Kind",
			"verbs.go:41:30: numeric-verb: example.com/verbs.Kind",
			"verbs.go:42:29: numeric-verb: example.com/verbs.Kind",
			"verbs.go:45:31: numeric-verb: example.com/verbs.Kind",
			// A verb after a malformed or out-of-range index formats nothing, and leaves
			// the next verb the first operand.
			"verbs.go:49:58: numeric-verb: example.com/verbs.Kind",
		}))
	})

	It("says which verb", func() {
		Expect(messages("verbs", engine.Config{})).To(HaveKeyWithValue("verbs.go:31:39",
			"numeric-verb: formats verbs.Kind under %d, which prints its ordinal rather than its member's name; use %v or %s"))
	})
})
