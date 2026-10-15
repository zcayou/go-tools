package importforms_test

import (
	"testing"

	"github.com/onsi/gomega"

	"importforms"
)

func TestNamed(t *testing.T) {
	gomega.Expect(importforms.Do()).To(gomega.HaveOccurred()) // want `^unidentified-error: HaveOccurred`
	gomega.Expect(importforms.Do()).To(gomega.Not(gomega.HaveOccurred()))

	g := gomega.NewWithT(t)
	g.Expect(importforms.Do()).NotTo(gomega.Succeed()) // want `^unidentified-error: a negated Succeed`

	var fresh gomega.Gomega = gomega.NewGomega(nil)
	fresh.Expect(importforms.Do()).To(gomega.HaveOccurred()) // want `^unidentified-error: HaveOccurred`
}
