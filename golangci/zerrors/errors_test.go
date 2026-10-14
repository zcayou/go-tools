package zerrors_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zerrors"
)

var _ = Describe("LoadError", func() {
	It("names the module root and what the loader said", func() {
		dir := fixtureDir("broken")

		_, err := runFixture("broken")

		Expect(err).To(MatchError(HavePrefix(
			"zerrors: loading " + dir + ": packages contain errors: example.com/broken: ")))
	})

	It("names the module root when the loader gave no reason", func() {
		Expect(&zerrors.LoadError{Root: "/m"}).To(MatchError("zerrors: loading /m failed"))
	})
})
