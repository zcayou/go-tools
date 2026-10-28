package split

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestInPackage(t *testing.T) {
	Expect(InPackage{}.Error()).To(Equal("in package"))
	Expect(Neither{}.Error()).To(HavePrefix("nei"))
}
