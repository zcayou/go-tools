package enum_test

import (
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/internal/enum"
)

// loaded is the fixture package, loaded by the first spec that asks for it.
var loaded *enum.Package

// fixture loads the enums fixture module's package with its test files, which
// is the variant a constant declared in a test file could reach.
func fixture() enum.Package {
	GinkgoHelper()

	if loaded != nil {
		return *loaded
	}
	dir, err := filepath.Abs(filepath.Join("testdata", "enums"))
	Expect(err).NotTo(HaveOccurred())
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo,
		Dir:   dir,
		Tests: true,
	}, ".")
	Expect(err).NotTo(HaveOccurred())
	for _, pkg := range pkgs {
		Expect(pkg.Errors).To(BeEmpty())
		if strings.HasSuffix(pkg.ID, ".test]") && pkg.Name == "enums" {
			loaded = &enum.Package{Fset: pkg.Fset, Files: pkg.Syntax, Types: pkg.Types, Info: pkg.TypesInfo}
			return *loaded
		}
	}
	Fail("no test variant of the fixture package")
	return enum.Package{}
}

func declared(pkg enum.Package, name string) *enum.Enum {
	GinkgoHelper()

	for _, e := range enum.Declared(pkg) {
		if e.Type.Obj().Name() == name {
			return e
		}
	}
	Fail("no enum " + name)
	return nil
}

func memberNames(e *enum.Enum) []string {
	names := make([]string, 0, len(e.Members))
	for _, member := range e.Members {
		names = append(names, member.Name())
	}
	return names
}
