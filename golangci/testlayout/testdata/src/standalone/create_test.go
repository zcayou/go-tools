// No source sits in this directory, so a spec file is named for the scenario
// it exercises rather than for a source file, and the package name carries no
// _test suffix because there is no package to be outside of.
package standalone

var _ = Describe("Create", func() {
	It("creates", func() {})
})
