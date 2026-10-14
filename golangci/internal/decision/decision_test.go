package decision_test

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/ssa"

	"github.com/zcayou/go-tools/golangci/internal/decision"
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

var _ = Describe("Find", func() {
	DescribeTable("finds each decision on a subject, however it is derived",
		func(name string, want []string) {
			Expect(decisions(name)).To(Equal(want))
		},
		Entry("a strings matcher", "Contains", []string{"11:25"}),
		Entry("a matcher on a transformation", "Lowered", []string{"15:26"}),
		Entry("a comparison with a slice", "Sliced", []string{"19:25"}),
		Entry("a comparison with a local one assignment makes a subject", "Local", []string{"27:13"}),
		Entry("the cases of a switch", "Switch", []string{"32:7", "34:7"}),
		Entry("a regexp method", "Pattern", []string{"41:28"}),
		Entry("a regexp function", "PatternFunc", []string{"45:29"}),
		Entry("a load of a local that escaped", "Escaped", []string{"52:12"}),
		Entry("a strings matcher against a rune", "Rune", []string{"80:29"}),
	)

	DescribeTable("finds no decision where none is made on content",
		func(name string) {
			Expect(decisions(name)).To(BeEmpty())
		},
		Entry("a comparison with the empty string", "Empty"),
		Entry("a comparison of two subjects", "Both"),
		Entry("an ordering", "Ordered"),
		Entry("a comparison of no subject", "Unrelated"),
		Entry("a strings matcher against a computed string", "Parsed"),
		Entry("a strings matcher against the empty string", "CountedEmpty"),
	)

	It("names the subject a decision was made on", func() {
		found := decision.Find(function("Lowered"), errorText)

		Expect(found).To(HaveLen(1))
		call, ok := found[0].Subject.(*ssa.Call)
		Expect(ok).To(BeTrue())
		Expect(call.Call.Method.Name()).To(Equal("Error"))
	})
})

// errorText recognizes the result of calling Error on an error.
func errorText(v ssa.Value) bool {
	call, ok := v.(*ssa.Call)
	return ok && call.Call.Method != nil && call.Call.Method.Name() == "Error"
}

func function(name string) *ssa.Function {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join("testdata", "text"))
	Expect(err).NotTo(HaveOccurred())
	module, err := wholemodule.Load(dir, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(module.Packages).To(HaveLen(1))
	fn := module.Program().Packages[module.Packages[0].Types].Func(name)
	Expect(fn).NotTo(BeNil())
	return fn
}

// decisions renders where the fixture function name decides on error text,
// as line:column.
func decisions(name string) []string {
	GinkgoHelper()

	fn := function(name)
	all := decision.Find(fn, errorText)
	out := make([]string, 0, len(all))
	for _, found := range all {
		pos := fn.Prog.Fset.Position(found.Pos)
		out = append(out, fmt.Sprintf("%d:%d", pos.Line, pos.Column))
	}
	return out
}
