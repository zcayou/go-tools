package customrendering_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"customrendering"
)

func TestErrors(t *testing.T) {
	Expect(customrendering.Do()).To(MatchError("failure")) // want `^text-assertion`
}
