package pinning_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"pinning"
)

func TestPlatformRendering(t *testing.T) {
	Expect(pinning.Do()).To(MatchError("failure"))
}
