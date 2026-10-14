package hidden_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"example.com/apiscope/internal/hidden"
)

func TestDo(t *testing.T) {
	Expect(hidden.Do()).To(HaveOccurred())
}
