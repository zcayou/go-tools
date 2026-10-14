package wholemodule_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/ssa"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

var _ = Describe("VariadicArgs", func() {
	It("returns the operands a call packs into its variadic parameter, in order", func() {
		args, ok := wholemodule.VariadicArgs(sprintCall("Hello"))

		Expect(ok).To(BeTrue())
		Expect(args).To(HaveLen(2))
		Expect(args).To(HaveEach(BeAssignableToTypeOf(&ssa.MakeInterface{})))
		Expect(operand(args[0])).To(Equal(`"a":string`))
		Expect(operand(args[1])).To(Equal("1:int"))
	})

	It("returns no operands for a call that passes none", func() {
		args, ok := wholemodule.VariadicArgs(sprintCall("None"))

		Expect(ok).To(BeTrue())
		Expect(args).To(BeEmpty())
	})

	It("cannot see the operands of a slice passed with ...", func() {
		_, ok := wholemodule.VariadicArgs(sprintCall("Spread"))

		Expect(ok).To(BeFalse())
	})
})

// operand renders what a variadic operand converts to an interface.
func operand(arg ssa.Value) string {
	conversion, ok := arg.(*ssa.MakeInterface)
	if !ok {
		return ""
	}
	return conversion.X.String()
}

// sprintCall returns the call to fmt.Sprint in the plain fixture's function
// of that name.
func sprintCall(name string) *ssa.CallCommon {
	GinkgoHelper()

	module, err := wholemodule.Load(fixtureDir("plain"), nil)
	Expect(err).NotTo(HaveOccurred())
	fn := module.Program().Packages[libPackage(module).Types].Func(name)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if call, ok := instr.(*ssa.Call); ok {
				return call.Common()
			}
		}
	}
	Fail("no call in " + name)
	return nil
}
