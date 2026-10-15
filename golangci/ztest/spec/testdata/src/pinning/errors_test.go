package pinning_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"pinning"
)

func TestRendering(t *testing.T) {
	Expect(pinning.Do()).To(MatchError("failure"))
	Expect(pinning.Do().Error()).To(Equal("failure"))
}
