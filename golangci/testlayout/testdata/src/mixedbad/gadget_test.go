// Both test packages register specs and neither holds an adapter. The first
// spec-bearing file in the directory carries the diagnostic, so the internal
// package's pass stays quiet about a problem this one reports.
package mixedbad_test // want `ginkgo-adapter-missing: this package registers Ginkgo specs but no file calls RunSpecs`

import "mixedbad"

var _ = mixedbad.Describe("Gadget", func() {})
