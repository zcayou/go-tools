package wholemodule_test

import (
	"go/types"
	"path/filepath"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

var _ = Describe("Load", func() {
	It("loads the module's packages from source, without their tests", func() {
		module, err := wholemodule.Load(fixtureDir("plain"), nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(module.Root).To(Equal(fixtureDir("plain")))
		Expect(paths(module.Packages)).To(ConsistOf(
			"example.com/plain", "example.com/plain/lib", "example.com/plain/internal/hidden"))
		for _, pkg := range module.Packages {
			Expect(pkg.Syntax).NotTo(BeEmpty())
			Expect(pkg.TypesInfo).NotTo(BeNil())
			for _, file := range pkg.GoFiles {
				Expect(file).NotTo(HaveSuffix("_test.go"))
			}
		}
	})

	It("reads dependencies from export data, which declare nothing of the module's", func() {
		module, err := wholemodule.Load(fixtureDir("plain"), nil)
		Expect(err).NotTo(HaveOccurred())

		lib := libPackage(module)
		fmtPkg := lib.Types.Imports()[0]

		Expect(fmtPkg.Path()).To(Equal("fmt"))
		Expect(module.Package(fmtPkg)).To(BeNil())
		Expect(module.Declares(fmtPkg.Scope().Lookup("Sprint"))).To(BeFalse())
		Expect(module.Declares(lib.Types.Scope().Lookup("Hello"))).To(BeTrue())
	})

	It("loads a module once per root and build-tag set", func() {
		first, err := wholemodule.Load(fixtureDir("plain"), []string{"b", "a"})
		Expect(err).NotTo(HaveOccurred())
		second, err := wholemodule.Load(fixtureDir("plain"), []string{"a", "b"})
		Expect(err).NotTo(HaveOccurred())
		untagged, err := wholemodule.Load(fixtureDir("plain"), nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(second).To(BeIdenticalTo(first))
		Expect(untagged).NotTo(BeIdenticalTo(first))
	})

	It("knows every package the module imports, directly or transitively", func() {
		module, err := wholemodule.Load(fixtureDir("plain"), nil)
		Expect(err).NotTo(HaveOccurred())

		imported := module.ImportedPaths()

		Expect(imported).To(ContainElements("fmt", "unicode/utf8"))
		Expect(imported).NotTo(ContainElement("example.com/plain/lib"))
		Expect(imported).NotTo(ContainElement("net/http"))
		Expect(slices.IsSorted(imported)).To(BeTrue())
	})

	It("returns the type information the load already holds for an imported package", func() {
		module, err := wholemodule.Load(fixtureDir("plain"), nil)
		Expect(err).NotTo(HaveOccurred())

		fmtPkg := module.ImportedPackage("fmt")

		Expect(fmtPkg).To(BeIdenticalTo(libPackage(module).Types.Imports()[0]))
	})

	It("holds a package only imported transitively in the load's own type universe", func() {
		module, err := wholemodule.Load(fixtureDir("plain"), nil)
		Expect(err).NotTo(HaveOccurred())
		atomicPkg := module.ImportedPackage("sync/atomic")

		syncPkg := module.ImportedPackage("sync")

		Expect(syncPkg.Path()).To(Equal("sync"))
		Expect(syncPkg.Complete()).To(BeTrue())
		once, ok := syncPkg.Scope().Lookup("Once").Type().Underlying().(*types.Struct)
		Expect(ok).To(BeTrue())
		var fieldPackages []*types.Package
		for field := range once.Fields() {
			if named, ok := field.Type().(*types.Named); ok {
				fieldPackages = append(fieldPackages, named.Obj().Pkg())
			}
		}
		Expect(fieldPackages).To(ContainElement(BeIdenticalTo(atomicPkg)))
	})

	It("completes, before returning, a package export data refers to only in part", func() {
		module, err := wholemodule.Load(fixtureDir("plain"), nil)
		Expect(err).NotTo(HaveOccurred())

		fmtPkg := libPackage(module).Types.Imports()[0]
		fprint, ok := fmtPkg.Scope().Lookup("Fprint").Type().(*types.Signature)
		Expect(ok).To(BeTrue())
		writer, ok := fprint.Params().At(0).Type().(*types.Named)
		Expect(ok).To(BeTrue())
		held := writer.Obj().Pkg()

		Expect(held.Complete()).To(BeTrue())
		Expect(held.Scope().Lookup("SectionReader")).NotTo(BeNil())
		Expect(module.ImportedPackage("io")).To(BeIdenticalTo(held))
	})

	It("holds every imported package complete", func() {
		module, err := wholemodule.Load(fixtureDir("plain"), nil)
		Expect(err).NotTo(HaveOccurred())

		for _, path := range module.ImportedPaths() {
			Expect(module.ImportedPackage(path)).To(HaveField("Complete()", BeTrue()), path)
		}
	})

	It("returns nothing for a package the module does not import", func() {
		module, err := wholemodule.Load(fixtureDir("plain"), nil)
		Expect(err).NotTo(HaveOccurred())

		Expect(module.ImportedPackage("net/http")).To(BeNil())
	})

	It("fails on a package that does not type check", func() {
		_, err := wholemodule.Load(fixtureDir("broken"), nil)

		Expect(err).To(BeAssignableToTypeOf(&wholemodule.LoadError{}))
		Expect(err).To(HaveField("Errors", HaveKey("example.com/broken")))
	})

	It("builds the module's SSA program once, with every module package in it", func() {
		module, err := wholemodule.Load(fixtureDir("plain"), nil)
		Expect(err).NotTo(HaveOccurred())

		prog := module.Program()

		Expect(module.Program()).To(BeIdenticalTo(prog))
		Expect(prog.Packages).To(HaveLen(len(module.Packages)))
		hello := prog.Packages[libPackage(module).Types].Func("Hello")
		Expect(hello).NotTo(BeNil())
		Expect(hello.Blocks).NotTo(BeEmpty())
	})
})

var _ = Describe("LoadError", func() {
	It("names what each failed package's loader said", func() {
		err := &wholemodule.LoadError{Errors: map[string][]packages.Error{
			"example.com/b": {{Pos: "b.go:1:1", Msg: "undefined: y"}},
			"example.com/a": {{Pos: "a.go:1:1", Msg: "undefined: x"}},
		}}

		Expect(err).To(MatchError(
			"packages contain errors: example.com/a: a.go:1:1: undefined: x; example.com/b: b.go:1:1: undefined: y"))
	})

	It("names the loader's own failure", func() {
		_, err := wholemodule.Load(filepath.Join(anchorlessDir(), "missing"), nil)

		Expect(err).To(MatchError(HavePrefix("loading packages: ")))
	})

	It("says when the loader gave no reason, naming the imported package it was reading", func() {
		Expect(&wholemodule.LoadError{}).To(MatchError("loading packages failed, and the loader said nothing of why"))
		Expect(&wholemodule.LoadError{Package: "example.com/x"}).
			To(MatchError("loading example.com/x failed, and the loader said nothing of why"))
	})
})

func paths(pkgs []*packages.Package) []string {
	out := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		out = append(out, pkg.PkgPath)
	}
	return out
}
