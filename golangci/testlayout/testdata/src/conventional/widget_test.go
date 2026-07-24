package conventional_test

import "conventional"

var _ = Describe("Widget", func() {
	It("returns one", func() {
		_ = conventional.Widget()
	})
})
