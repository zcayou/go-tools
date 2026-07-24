package adaptermissing_test // want `ginkgo-adapter-missing: this package registers Ginkgo specs but no file calls RunSpecs`

func Describe(text string, body func()) bool { body(); return true }

var _ = Describe("Widget", func() {})
