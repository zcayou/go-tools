package wholemodule_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

func fixtureDir(name string) string {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())
	return dir
}

// anchorlessDir returns a directory with no go.mod at or above it, which
// is what a repository golangci-lint cannot anchor looks like from a linter.
func anchorlessDir() string {
	GinkgoHelper()

	dir, err := os.MkdirTemp("", "wholemodule-anchorless")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { Expect(os.RemoveAll(dir)).To(Succeed()) })
	resolved, err := filepath.EvalSymlinks(dir)
	Expect(err).NotTo(HaveOccurred())
	Expect(filepath.Join(resolved, "go.mod")).NotTo(BeAnExistingFile())
	return resolved
}

func chdir(dir string) {
	GinkgoHelper()

	previous, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chdir(dir)).To(Succeed())
	DeferCleanup(func() { Expect(os.Chdir(previous)).To(Succeed()) })
}

// libPackage returns the plain fixture's lib package.
func libPackage(module *wholemodule.Module) *packages.Package {
	GinkgoHelper()

	const path = "example.com/plain/lib"
	for _, pkg := range module.Packages {
		if pkg.PkgPath == path {
			return pkg
		}
	}
	Fail("no package " + path)
	return nil
}

// testPackage returns the plain fixture's test package with that import path.
func testPackage(module *wholemodule.Module, path string) *packages.Package {
	GinkgoHelper()

	for _, pkg := range module.Tests {
		if pkg.PkgPath == path {
			return pkg
		}
	}
	Fail("no test package " + path)
	return nil
}
