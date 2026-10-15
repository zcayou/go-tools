package narrow_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"example.com/apiscope/narrow"
)

func TestDo(t *testing.T) {
	Expect(narrow.Do()).To(HaveOccurred())
}
