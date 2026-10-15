package unparsable_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"unparsable"
)

func TestReport(t *testing.T) {
	Expect(unparsable.Do()).To(MatchError("boom"))
}
