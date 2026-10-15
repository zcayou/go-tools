package pinning_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"pinning"
)

func TestElsewhere(t *testing.T) {
	Expect(pinning.Do()).To(MatchError("failure")) // want `^text-assertion`
}
