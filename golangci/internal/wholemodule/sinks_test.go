package wholemodule_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/ssa"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

var _ = Describe("SinkOf", func() {
	DescribeTable("says what a standard-library sink does with a value it never renders",
		func(function string, want wholemodule.Sink) {
			call, conversion := sinkCall(function)

			Expect(wholemodule.SinkOf(call, conversion)).To(Equal(want))
		},
		Entry("a context key", "ContextKey", wholemodule.SinkCompares),
		Entry("a context value", "ContextValue", wholemodule.SinkHolds),
		Entry("a key a context is asked for", "ContextLookup", wholemodule.SinkCompares),
		Entry("a sync.Map key", "MapKey", wholemodule.SinkCompares),
		Entry("a sync.Map value", "MapValue", wholemodule.SinkHolds),
		Entry("one value a sync.Map is given as key and value", "MapBoth", wholemodule.SinkCompares),
		Entry("a sync.Map compare-and-swap's old value", "MapOld", wholemodule.SinkCompares),
		Entry("an atomic.Value's stored value", "AtomicStore", wholemodule.SinkHolds),
		Entry("an atomic.Value compare-and-swap's old value", "AtomicOld", wholemodule.SinkCompares),
		Entry("a slice sort.Slice sorts", "Sorted", wholemodule.SinkHolds),
		Entry("a value reflect.TypeOf describes", "TypeOf", wholemodule.SinkHolds),
	)

	It("says nothing of a call that may render the value", func() {
		call, conversion := sinkCall("Printed")

		Expect(wholemodule.SinkOf(call, conversion)).To(BeZero())
	})

	DescribeTable("renders",
		func(sink wholemodule.Sink, token string) {
			Expect(sink.String()).To(Equal(token))
		},
		Entry("holding", wholemodule.SinkHolds, "holds"),
		Entry("comparing", wholemodule.SinkCompares, "compares"),
		Entry("a value no sink use holds", wholemodule.Sink(0), "Sink(0)"),
	)
})

// sinkCall returns the call the sinks fixture's function hands its Value to,
// and the conversion of that value to an interface.
func sinkCall(function string) (*ssa.CallCommon, ssa.Value) {
	GinkgoHelper()

	module, err := wholemodule.Load(fixtureDir("sinks"), nil)
	Expect(err).NotTo(HaveOccurred())
	prog := module.Program()
	var fn *ssa.Function
	for _, pkg := range prog.Packages {
		if pkg.Pkg.Path() == "example.com/sinks" {
			fn = pkg.Func(function)
		}
	}
	Expect(fn).NotTo(BeNil())
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			conversion, ok := instr.(*ssa.MakeInterface)
			if !ok || !strings.Contains(conversion.X.Type().String(), "sinks.Value") {
				continue
			}
			for _, ref := range *conversion.Referrers() {
				if call, ok := ref.(ssa.CallInstruction); ok {
					return call.Common(), conversion
				}
			}
		}
	}
	Fail("no conversion of a Value handed to a call in " + function)
	return nil, nil
}
