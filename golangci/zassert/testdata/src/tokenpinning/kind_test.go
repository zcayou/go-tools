package tokenpinning

import (
	"testing"

	. "github.com/onsi/gomega"
)

// The spec of the file declaring Kind pins its tokens, from inside
// the package.
func TestKindTokens(t *testing.T) {
	Expect(KindA.String()).To(Equal("a"))
}
