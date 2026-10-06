package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

var _ = Describe("buildProgram", func() {
	// main boxes a session[string], whose Run boxes an attempt[string]. x/tools
	// builds a runtime type's methods only when asked, so attempt[string].Go
	// exists only once session[string].Run has been built and read.
	const lazy = "(lazybodies.attempt[string]).Go"

	It("is needed: one visit of a freshly built program misses the nested body", func() {
		prog, _ := ssautil.Packages(loadModule("lazybodies"), ssa.InstantiateGenerics)
		prog.Build()

		Expect(functionNames(ssautil.AllFunctions(prog))).NotTo(ContainElement(lazy))
	})

	It("holds every body the program will build before anything reads it", func() {
		prog, _, err := engine.BuildProgram(loadModule("lazybodies"))
		Expect(err).NotTo(HaveOccurred())

		Expect(functionNames(prog.Functions())).To(ContainElement(lazy))
	})
})

// loadModule loads a whole-module fixture with the syntax and types the engine
// builds SSA from.
func loadModule(name string) []*packages.Package {
	GinkgoHelper()

	pkgs, err := packages.Load(&packages.Config{Mode: packages.LoadAllSyntax, Dir: fixtureDir(name)}, "./...")
	Expect(err).NotTo(HaveOccurred())
	return pkgs
}

func functionNames(funcs map[*ssa.Function]bool) []string {
	names := make([]string, 0, len(funcs))
	for fn := range funcs {
		names = append(names, fn.String())
	}
	return names
}
