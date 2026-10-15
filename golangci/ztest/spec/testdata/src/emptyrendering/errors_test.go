package emptyrendering_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"emptyrendering"
)

func TestErrors(t *testing.T) {
	Expect(emptyrendering.Do()).To(MatchError("failure")) // want `^text-assertion`
}
