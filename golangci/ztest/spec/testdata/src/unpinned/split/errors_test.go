package split_test // want `^unpinned-rendering: split\.Neither's rendering is not pinned; assert its exact text on a split\.Neither value in this file$`

import (
	"testing"

	. "github.com/onsi/gomega"

	"unpinned/split"
)

func TestExternal(t *testing.T) {
	Expect(split.Neither{}).NotTo(MatchError("neither"))
}
