package customadapter_test

import (
	. "github.com/onsi/ginkgo/v2"

	"customadapter"
)

var _ = Describe("Thing", func() {
	It("returns one", func() {
		_ = customadapter.Thing()
	})
})
