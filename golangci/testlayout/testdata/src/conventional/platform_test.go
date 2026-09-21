// platform_test.go has no platform.go beside it, only the build-constrained
// platform_linux.go, which is the source it is named for all the same.
package conventional_test

import . "github.com/onsi/ginkgo/v2"

var _ = Describe("Platform", func() {
	It("is named for a build-constrained source", func() {})
})
