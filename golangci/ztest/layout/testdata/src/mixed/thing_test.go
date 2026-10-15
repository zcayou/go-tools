package mixed

import . "github.com/onsi/ginkgo/v2"

var _ = Describe("Thing", func() {
	It("reaches an unexported identifier", func() {
		_ = thing()
	})
})
