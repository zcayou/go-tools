package thing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/onsi/gomega"

	"internalcase/internal/thing"
)

func TestThing(t *testing.T) {
	Expect(thing.Do()).To(HaveOccurred())
	Expect(thing.Do()).NotTo(Succeed())
	assert.Error(t, thing.Do())

	Expect(thing.Do()).To(MatchError("boom")) // want `^text-assertion: asserts an error's text`
}
