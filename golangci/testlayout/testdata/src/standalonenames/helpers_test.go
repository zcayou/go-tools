package standalonenames // want `specs-in-helper-file: a file named for its supporting role carries specs; move them to a file named \*_integration_test\.go` `ginkgo-adapter-missing: this package registers Ginkgo specs but no file calls RunSpecs, so none of them run; add suite_test\.go`

import . "github.com/onsi/ginkgo/v2"

var _ = Describe("Helpers", func() {})
