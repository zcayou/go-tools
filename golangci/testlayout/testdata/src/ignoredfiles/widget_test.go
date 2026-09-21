package ignoredfiles_test // want `ginkgo-adapter-missing: this package registers Ginkgo specs but no file calls RunSpecs`

import . "github.com/onsi/ginkgo/v2"

var _ = Describe("Widget", func() {})
