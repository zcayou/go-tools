package engine_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("numeric verbs", func() {
	// Kind's own String prints a value no member holds under %d, which is not
	// reported.
	It("reports an ordinal enum value under each verb that bypasses its String", func() {
		Expect(inFile(analyzeFixture("verbs", engine.Config{}), "verbs.go")).To(Equal([]string{
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

	It("follows fmt into what it descends into, as far as it goes", func() {
		Expect(analyzeFixture("verbs", engine.Config{})).To(ContainElements(
			// A slice's elements, a struct's fields exported or not, a map's keys,
			// and a pointer's target at the top level. A pointer to an enum prints
			// its address. A Stringer carrier does not stop %d, which calls no String,
			// but does stop %x, and a Formatter stops every verb.
			"reach.go:46:21: numeric-verb: example.com/verbs.Kind",
			"reach.go:47:21: numeric-verb: example.com/verbs.Kind",
			"reach.go:48:21: numeric-verb: example.com/verbs.Kind",
			"reach.go:49:21: numeric-verb: example.com/verbs.Kind",
			"reach.go:51:21: numeric-verb: example.com/verbs.Kind",
		))
		Expect(analyzeFixture("verbs", engine.Config{})).NotTo(ContainElements(
			HavePrefix("reach.go:50:"), HavePrefix("reach.go:52:"), HavePrefix("reach.go:53:"),
		))
	})

	It("judges every printer: vet's, the module's inferred to a fixed point, and a foreign one by its shape", func() {
		Expect(analyzeFixture("verbs", engine.Config{})).To(ContainElements(
			// A printf wrapper, one named for its message, one wrapping a wrapper,
			// an interface method every converted type implements with one,
			// and a foreign Infof. A print wrapper formats as %v.
			"printers.go:37:13: numeric-verb: example.com/verbs.Kind",
			"printers.go:38:13: numeric-verb: example.com/verbs.Kind",
			"printers.go:39:14: numeric-verb: example.com/verbs.Kind",
			"printers.go:40:7: string-bypass: example.com/verbs.Kind",
			"printers.go:44:25: numeric-verb: example.com/verbs.Kind",
			"printers.go:45:18: numeric-verb: example.com/verbs.Kind",
		))
		// A name ending in f that forwards nothing, one handing a printf printer
		// a format of its own, one assigning to its operands, and a foreign
		// print-kind method no body shows.
		Expect(analyzeFixture("verbs", engine.Config{})).NotTo(ContainElements(
			HavePrefix("printers.go:41:"), HavePrefix("printers.go:42:"),
			HavePrefix("printers.go:43:"), HavePrefix("printers.go:46:"),
		))
	})

	It("reports a generic body's operand where an instantiation makes it an enum", func() {
		Expect(analyzeFixture("verbs", engine.Config{})).To(ContainElement(
			"generic.go:5:44: numeric-verb: example.com/verbs.Kind"))
	})
})

var _ = Describe("string bypasses", func() {
	It("reports an enum formatted for its name where fmt calls no String", func() {
		reported := analyzeFixture("verbs", engine.Config{})

		// Beneath an unexported field, at any depth, and through a print printer;
		// a uint8 slice under %q. A Stringer carrier is named by its own String; %v
		// of the slice, and %#v, which asks for Go syntax, are clean.
		Expect(reported).To(ContainElements(
			"reach.go:57:21: string-bypass: example.com/verbs.Kind",
			"reach.go:58:22: string-bypass: example.com/verbs.Kind",
			"reach.go:59:14: string-bypass: example.com/verbs.Kind",
			"reach.go:60:21: string-bypass: example.com/verbs.Flag",
		))
		Expect(reported).NotTo(ContainElements(
			HavePrefix("reach.go:61:"), HavePrefix("reach.go:62:"),
			HavePrefix("reach.go:63:"), HavePrefix("reach.go:64:"),
		))
	})

	It("names the field fmt stops calling methods at, or the slice it prints as bytes", func() {
		found := messages("verbs", engine.Config{})

		Expect(found).To(HaveKeyWithValue("reach.go:59:14", "string-bypass: formats verbs.Kind beneath "+
			"the unexported field Wrapper.inner, where fmt calls no String; export the field, or format the field itself"))
		Expect(found).To(HaveKeyWithValue("reach.go:60:21", "string-bypass: formats verbs.Flag as the elements "+
			"of []verbs.Flag under %q, which fmt prints as raw bytes; format it with %v"))
	})
})

// inFile keeps the rendered findings sitting in the named file of the fixture.
func inFile(findings []string, name string) []string {
	var kept []string
	for _, finding := range findings {
		if strings.HasPrefix(finding, name+":") {
			kept = append(kept, finding)
		}
	}
	return kept
}
