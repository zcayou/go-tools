package layout_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/ztest/layout"
)

var _ = Describe("Kind", func() {
	It("names each kind by its settings block", func() {
		Expect([]string{
			layout.KindWhitebox.String(), layout.KindBlackbox.String(), layout.KindStandalone.String(),
			layout.Kind(0).String(),
		}).To(Equal([]string{"whitebox", "blackbox", "standalone", "Kind(0)"}))
	})
})
