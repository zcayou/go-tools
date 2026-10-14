package enum_test

import (
	"go/constant"
	"go/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/enum"
)

var _ = Describe("Declared", func() {
	var pkg enum.Package
	BeforeEach(func() { pkg = fixture() })

	// Flags and Bit are flag sets, Size a quantity, and Generic generic: none
	// is an enum.
	It("finds each type its package declares constants of, outside test files", func() {
		enums := enum.Declared(pkg)
		names := make([]string, 0, len(enums))
		for _, e := range enums {
			names = append(names, e.Type.Obj().Name())
		}

		Expect(names).To(Equal([]string{
			"Ambiguous", "Bounded", "Chain", "CommaOk", "Constant", "Counted", "Delegating", "Explicit",
			"Formatted", "Holey", "Keyed", "Local", "MapLen", "Mapped", "Mode", "Nameless", "Offset", "Ordered",
			"Panicky", "Pointer", "Published", "Ratio", "Runs", "Single", "Sparse", "Special", "Switch", "Table",
			"Tagless", "Through", "Unbounded", "Unnamed", "Wrapped",
		}))
	})

	DescribeTable("leaves out a count sentinel, and only a count sentinel",
		func(name string, want []string) {
			Expect(memberNames(declared(pkg, name))).To(Equal(want))
		},
		Entry("one sizing an array", "Bounded", []string{"BoundedA", "BoundedB"}),
		Entry("one an ordering compares against", "Ordered", []string{"OrderedA", "OrderedB"}),
		Entry("an exported one", "Published", []string{"PublishedA", "PublishedB", "PublishedCount"}),
		Entry("one nothing bounds by", "Unbounded", []string{"UnboundedA", "unboundedB"}),
		Entry("one writing its own value", "Explicit", []string{"ExplicitA", "explicitMax"}),
		Entry("one counted by adding", "Counted", []string{"CountedA", "CountedB"}),
	)

	It("lists the members in declaration order, re-exports included", func() {
		Expect(memberNames(declared(pkg, "Switch"))).To(Equal([]string{"SwitchA", "SwitchB", "SwitchC", "SwitchAlias"}))
	})

	It("judges membership by value", func() {
		e := declared(pkg, "Switch")

		Expect(e.Values()).To(HaveLen(3))
		Expect(e.Member(constant.MakeInt64(2))).To(BeTrue())
		Expect(e.Member(constant.MakeInt64(99))).To(BeFalse())
		Expect(e.Member(constant.MakeString("a"))).To(BeFalse())
	})

	It("tells ordinal, textual and other enums apart", func() {
		Expect(declared(pkg, "Switch").Ordinal()).To(BeTrue())
		Expect(declared(pkg, "Offset").Ordinal()).To(BeTrue())
		Expect(declared(pkg, "Mode").Textual()).To(BeTrue())
		Expect(declared(pkg, "Mode").Ordinal()).To(BeFalse())
		Expect(declared(pkg, "Ratio").Ordinal()).To(BeFalse())
		Expect(declared(pkg, "Ratio").Textual()).To(BeFalse())
	})
})

var _ = Describe("Name", func() {
	var pkg enum.Package
	BeforeEach(func() { pkg = fixture() })

	It("is the String() string method on a value receiver", func() {
		name := enum.Name(declared(pkg, "Switch").Type)

		Expect(name).NotTo(BeNil())
		Expect(name.Name()).To(Equal("String"))
	})

	It("is nothing for a type that declares none, or only on a pointer receiver", func() {
		Expect(enum.Name(declared(pkg, "Nameless").Type)).To(BeNil())
		Expect(enum.Name(declared(pkg, "Pointer").Type)).To(BeNil())
	})

	It("is nothing for a String of another signature", func() {
		named := types.NewNamed(types.NewTypeName(0, pkg.Types, "Odd", nil), types.Typ[types.Int], nil)
		sig := types.NewSignatureType(types.NewVar(0, pkg.Types, "o", named), nil, nil, nil,
			types.NewTuple(types.NewVar(0, pkg.Types, "", types.Typ[types.Int])), false)
		named.AddMethod(types.NewFunc(0, pkg.Types, "String", sig))

		Expect(enum.Name(named)).To(BeNil())
	})
})
