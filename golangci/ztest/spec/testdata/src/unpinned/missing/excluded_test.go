//go:build integration

package missing_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"unpinned/missing"
)

func TestExcluded(t *testing.T) {
	Expect((&missing.Excluded{}).Error()).To(Equal("excluded"))
}
