package testlayout_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/testlayout"
)

var _ = Describe("Platform file names", func() {
	It("recognize every GOOS go/build reads out of a name, and unix", func() {
		Expect(slices.Collect(maps.Keys(testlayout.GOOSValues))).To(
			ConsistOf(append(toolchainNames("KnownOS"), "unix")))
	})

	It("recognize every GOARCH go/build reads out of a name", func() {
		Expect(slices.Collect(maps.Keys(testlayout.GOARCHValues))).To(
			ConsistOf(toolchainNames("KnownArch")))
	})
})

// toolchainNames reads the keys of the named map in the toolchain's
// internal/syslist, which go/build matches file names against and nothing
// exported carries.
func toolchainNames(table string) []string {
	GinkgoHelper()

	Expect(build.Default.GOROOT).NotTo(BeEmpty(), "the test binary knows no GOROOT to read syslist from")
	path := filepath.Join(build.Default.GOROOT, "src", "internal", "syslist", "syslist.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	Expect(err).NotTo(HaveOccurred())

	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || value.Names[0].Name != table || len(value.Values) != 1 {
				continue
			}
			literal, ok := value.Values[0].(*ast.CompositeLit)
			Expect(ok).To(BeTrue(), "%s is not a map literal", table)
			for _, elt := range literal.Elts {
				entry, ok := elt.(*ast.KeyValueExpr)
				Expect(ok).To(BeTrue(), "%s has an element that is not a key and value", table)
				key, ok := entry.Key.(*ast.BasicLit)
				Expect(ok).To(BeTrue(), "%s has a key that is not a literal", table)
				name, err := strconv.Unquote(key.Value)
				Expect(err).NotTo(HaveOccurred())
				names = append(names, name)
			}
		}
	}
	Expect(names).NotTo(BeEmpty(), "%s declares no %s", path, table)
	return names
}
