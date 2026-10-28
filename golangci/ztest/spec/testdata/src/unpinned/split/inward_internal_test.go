package split // want `^unpinned-rendering: split\.Inward's rendering is not pinned; assert its exact text on a split\.Inward value in this file$`

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestInward(t *testing.T) {
	Expect(Inward{}).To(MatchError(ContainSubstring("in")))
}
