package deadcode_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// anchorlessDir returns a directory with no go.mod at or above it, which
// is what a repository golangci-lint cannot anchor looks like from the plugin.
func anchorlessDir() string {
	GinkgoHelper()

	dir, err := os.MkdirTemp("", "deadcode-anchorless")
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
