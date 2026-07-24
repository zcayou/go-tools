package mixedbad

var _ = Describe("Thing", func() {
	It("reaches an unexported identifier", func() {
		_ = thing()
	})
})
