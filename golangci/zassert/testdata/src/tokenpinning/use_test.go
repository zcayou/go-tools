package tokenpinning_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"tokenpinning"
)

func TestUse(t *testing.T) {
	Expect(tokenpinning.KindA.String()).To(Equal("a")) // want `^token-assertion: asserts the rendered token of tokenpinning.Kind`
}
