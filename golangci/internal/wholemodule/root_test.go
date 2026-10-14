package wholemodule_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

var _ = Describe("ModuleRoot", func() {
	It("walks up from the working directory to the nearest go.mod", func() {
		dir := fixtureDir("plain")
		chdir(filepath.Join(dir, "internal", "hidden"))

		root, err := wholemodule.ModuleRoot()

		Expect(err).NotTo(HaveOccurred())
		Expect(root).To(Equal(dir))
	})

	It("names the directory it started from when no go.mod is above it", func() {
		dir := anchorlessDir()
		chdir(dir)

		_, err := wholemodule.ModuleRoot()

		Expect(err).To(MatchError(&wholemodule.NoModuleError{Dir: dir}))
		Expect(err).To(MatchError("no go.mod at or above " + dir))
	})

	It("renders a value naming no directory", func() {
		Expect(&wholemodule.NoModuleError{}).To(MatchError("no go.mod at or above the working directory"))
	})
})
