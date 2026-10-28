package unpinned_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"unpinned/unpinned"
)

func TestOther(t *testing.T) {
	unpinned.Other()
	Expect(unpinned.Elsewhere{}.Error()).To(Equal("elsewhere")) // want `^text-assertion`
}
