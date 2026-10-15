package conventional_test

import (
	. "github.com/onsi/ginkgo/v2"

	"conventional"
)

var _ = Describe("Widget", func() {
	It("returns one", func() {
		_ = conventional.Widget()
	})
})
