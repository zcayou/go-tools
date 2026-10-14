package wide_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"example.com/apiscope/wide"
)

func TestDo(t *testing.T) {
	Expect(wide.Do()).To(HaveOccurred())
}
