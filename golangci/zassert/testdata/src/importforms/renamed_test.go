package importforms_test

import (
	"testing"

	gm "github.com/onsi/gomega"

	"importforms"
)

func TestRenamed(t *testing.T) {
	gm.Ω(importforms.Do()).Should(gm.HaveOccurred()) // want `^unidentified-error: HaveOccurred`
	gm.Ω(importforms.Do()).ShouldNot(gm.HaveOccurred())
}
