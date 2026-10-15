package tokenpinning_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"tokenpinning"
)

// The spec of the file declaring String pins Kind's tokens too.
func TestStringTokens(t *testing.T) {
	Expect(tokenpinning.KindB.String()).To(Equal("b"))
}
