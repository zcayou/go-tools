package customadapter_test

import "customadapter"

var _ = Describe("Thing", func() {
	It("returns one", func() {
		_ = customadapter.Thing()
	})
})
