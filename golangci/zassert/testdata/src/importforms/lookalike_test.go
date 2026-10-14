package importforms_test

import (
	"testing"

	"importforms"
)

type checks struct{}

func (checks) Expect(actual any) checks   { return checks{} }
func (checks) To(matcher int) bool        { return true }
func (checks) HaveOccurred() int          { return 0 }
func (checks) MatchError(text string) int { return 0 }

func HaveOccurred() int { return 0 }

func MatchError(text string) int { return 0 }

func TestLookalikes(t *testing.T) {
	var c checks
	c.Expect(importforms.Do()).To(HaveOccurred())
	c.Expect(importforms.Do()).To(c.HaveOccurred())
	c.Expect(importforms.Do()).To(MatchError("boom"))
	c.Expect(importforms.Do()).To(c.MatchError("boom"))
}
