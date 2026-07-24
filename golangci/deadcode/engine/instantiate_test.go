package engine_test

import (
	"fmt"
	"go/types"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

var _ = Describe("instantiations", func() {
	It("records the concrete vectors written at instantiation sites", func() {
		pkg := typecheckPackages(`package direct

type Box[T any] struct{}

func Use[T any]() {}

func Infer[T any](v T) {}

func run() {
	Use[int]()
	Use[int]()
	Use[string]()
	Infer(1.5)
	var _ Box[bool]
}
`)[0]

		inst := engine.NewInstantiations([]*packages.Package{pkg})

		Expect(vectorStrings(inst.Vectors(lookup[*types.Func](pkg, "Use")))).To(Equal([]string{"int", "string"}))
		Expect(vectorStrings(inst.Vectors(lookup[*types.Func](pkg, "Infer")))).To(Equal([]string{"float64"}))
		Expect(vectorStrings(inst.Vectors(lookup[*types.TypeName](pkg, "Box")))).To(Equal([]string{"bool"}))
	})

	It("resolves a parametric vector through the generics enclosing it", func() {
		// bottom is three levels below anything concrete, and the vectors inside
		// the methods reference the methods' own receiver type parameters.
		pkg := typecheckPackages(`package transitive

func bottom[T any]() {}

func middle[U any]() { bottom[U]() }

type Wrap[V any] struct{}

func (Wrap[V]) call() { middle[V]() }

func (*Wrap[V]) direct() { bottom[V]() }

func run() {
	Wrap[int]{}.call()
	var w Wrap[string]
	w.call()
	w2 := Wrap[bool]{}
	w2.direct()
}
`)[0]

		inst := engine.NewInstantiations([]*packages.Package{pkg})

		Expect(vectorStrings(inst.Vectors(lookup[*types.Func](pkg, "middle")))).To(Equal([]string{"bool", "int", "string"}))
		Expect(vectorStrings(inst.Vectors(lookup[*types.Func](pkg, "bottom")))).To(Equal([]string{"bool", "int", "string"}))
	})

	It("reads instantiations from dependencies the analyzed packages import", func() {
		pkgs := typecheckPackages(`package dep

func Bottom[T any]() {}

func Middle[U any]() { Bottom[U]() }
`, `package app

import "dep"

func run() { dep.Middle[float64]() }
`)

		inst := engine.NewInstantiations(pkgs[1:])

		Expect(vectorStrings(inst.Vectors(lookup[*types.Func](pkgs[0], "Bottom")))).To(Equal([]string{"float64"}))
	})

	It("resolves a vector written inside a generic type declaration", func() {
		pkg := typecheckPackages(`package typespec

type Box[T any] struct{}

type Pair[W any] struct {
	first  Box[W]
	second Box[W]
}

func run() {
	var _ Pair[float32]
}
`)[0]

		inst := engine.NewInstantiations([]*packages.Package{pkg})

		Expect(vectorStrings(inst.Vectors(lookup[*types.TypeName](pkg, "Box")))).To(Equal([]string{"float32"}))
	})

	It("yields nothing for a generic never instantiated with concrete arguments", func() {
		pkg := typecheckPackages(`package unresolved

func helper[T any]() {}

func generic[U any]() { helper[U]() }
`)[0]

		inst := engine.NewInstantiations([]*packages.Package{pkg})

		Expect(inst.Vectors(lookup[*types.Func](pkg, "helper"))).To(BeEmpty())
		Expect(inst.Vectors(lookup[*types.Func](pkg, "generic"))).To(BeEmpty())
	})

	It("caps an object's vectors after sorting them, so the drop is deterministic", func() {
		lines := make([]string, 0, engine.MaxResolvedVectors+15)
		lines = append(lines, "package capped", "", "func use[T any]() {}", "", "func run() {")
		expected := make([]string, 0, engine.MaxResolvedVectors+8)
		for i := range engine.MaxResolvedVectors + 8 {
			lines = append(lines, fmt.Sprintf("\tuse[[%d]int]()", i+1))
			expected = append(expected, fmt.Sprintf("[%d]int", i+1))
		}
		lines = append(lines, "}", "")
		slices.Sort(expected)

		pkg := typecheckPackages(strings.Join(lines, "\n"))[0]
		inst := engine.NewInstantiations([]*packages.Package{pkg})

		Expect(vectorStrings(inst.Vectors(lookup[*types.Func](pkg, "use")))).To(Equal(expected[:engine.MaxResolvedVectors]))
	})
})

var _ = Describe("substitute", func() {
	var (
		pkg    *packages.Package
		shapes *types.Named
		fields *types.Struct
		env    map[*types.TypeParam]types.Type
	)

	BeforeEach(func() {
		pkg = typecheckPackages(`package shapes

type Box[T any] struct{}

type Shapes[T comparable] struct {
	param     T
	pointer   *T
	slice     []T
	array     [4]T
	channel   chan<- T
	mapping   map[T]*T
	function  func(T) (T, error)
	structure struct {
		V T ` + "`tagged`" + `
	}
	iface interface{ Apply(T) error }
	named Box[T]
	boxed Box[[]T]
}

type Constraint[T any] interface{ ~[]T | string }

type Closed interface{ int | string }
`)[0]
		var ok bool
		shapes, ok = lookup[*types.TypeName](pkg, "Shapes").Type().(*types.Named)
		Expect(ok).To(BeTrue())
		fields, ok = shapes.Underlying().(*types.Struct)
		Expect(ok).To(BeTrue())
		env = map[*types.TypeParam]types.Type{shapes.TypeParams().At(0): types.Typ[types.Int]}
	})

	DescribeTable("rewrites every supported shape",
		func(field, expected string) {
			substituted, ok := engine.Substitute(fieldType(fields, field), env)

			Expect(ok).To(BeTrue())
			Expect(substituted.String()).To(Equal(expected))
		},
		Entry("a type parameter", "param", "int"),
		Entry("a pointer", "pointer", "*int"),
		Entry("a slice", "slice", "[]int"),
		Entry("an array", "array", "[4]int"),
		Entry("a channel", "channel", "chan<- int"),
		Entry("a map", "mapping", "map[int]*int"),
		Entry("a signature", "function", "func(int) (int, error)"),
		Entry("a struct, tags kept", "structure", `struct{V int "tagged"}`),
		Entry("an interface", "iface", "interface{Apply(int) error}"),
		Entry("a named instantiation", "named", "shapes.Box[int]"),
		Entry("a parametric argument of a named instantiation", "boxed", "shapes.Box[[]int]"),
	)

	It("returns a type mentioning no type parameter unchanged, whatever its shape", func() {
		closed := lookup[*types.TypeName](pkg, "Closed").Type().Underlying()

		substituted, ok := engine.Substitute(closed, env)

		Expect(ok).To(BeTrue())
		Expect(substituted).To(BeIdenticalTo(closed))
	})

	It("refuses a free type parameter the environment does not bind", func() {
		_, ok := engine.Substitute(fieldType(fields, "pointer"), nil)

		Expect(ok).To(BeFalse())
	})

	It("refuses a parametric shape it cannot rebuild, union terms among them", func() {
		constraint, ok := lookup[*types.TypeName](pkg, "Constraint").Type().(*types.Named)
		Expect(ok).To(BeTrue())
		bound := map[*types.TypeParam]types.Type{constraint.TypeParams().At(0): types.Typ[types.Int]}

		_, ok = engine.Substitute(constraint.Underlying(), bound)

		Expect(ok).To(BeFalse())
	})
})

// vectorStrings renders vectors the way expectations name them: one string per
// vector, its type arguments comma separated.
func vectorStrings(vectors [][]types.Type) []string {
	rendered := make([]string, len(vectors))
	for i, vector := range vectors {
		parts := make([]string, len(vector))
		for j, typ := range vector {
			parts[j] = typ.String()
		}
		rendered[i] = strings.Join(parts, ", ")
	}
	return rendered
}

// fieldType finds a struct field's type by name.
func fieldType(strct *types.Struct, name string) types.Type {
	GinkgoHelper()

	for field := range strct.Fields() {
		if field.Name() == name {
			return field.Type()
		}
	}
	Fail("no field named " + name)
	return nil
}
