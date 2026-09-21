// dsl/core re-exports the root package's builders, so dot-importing it alone
// is as much Ginkgo as dot-importing the root.
package ginkgoimports_test

import . "github.com/onsi/ginkgo/v2/dsl/core"

var _ = Context("Gamma", func() {})
