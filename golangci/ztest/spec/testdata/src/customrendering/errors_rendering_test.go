package customrendering_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"customrendering"
)

func TestRendering(t *testing.T) {
	Expect(customrendering.Do()).To(MatchError("failure"))
}
