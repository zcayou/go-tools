package unpinned_test // want `^unpinned-rendering: unpinned\.ErrorTyped's rendering is not pinned; assert its exact text on a unpinned\.ErrorTyped value in this file$` `unpinned\.Prefixed's rendering is not pinned` `unpinned\.Negated's rendering is not pinned` `unpinned\.Ored's rendering is not pinned` `unpinned\.Elsewhere's rendering is not pinned` `unpinned\.Busy's rendering is not pinned` `unpinned\.Calm's rendering is not pinned`

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	"unpinned/unpinned"
)

func TestRenderings(t *testing.T) {
	var err error = &unpinned.ErrorTyped{}
	Expect(err.Error()).To(Equal("error typed"))
	Expect(err).To(MatchError("error typed"))
	assert.EqualError(t, err, "error typed")

	Expect(unpinned.Prefixed{}.Error()).To(HavePrefix("pre"))
	Expect(unpinned.Prefixed{}).To(MatchError(HavePrefix("pre")))

	Expect(unpinned.Negated{}.Error()).NotTo(Equal("negated"))
	Expect(unpinned.Negated{}.Error()).To(Not(Equal("negated")))
	Expect(unpinned.Negated{}).ToNot(MatchError("negated"))
	assert.NotEqual(t, "negated", unpinned.Negated{}.Error())

	Expect(unpinned.Ored{}.Error()).To(Or(Equal("ored"), Equal("or")))
	Expect(unpinned.Ored{}).To(SatisfyAny(MatchError("ored")))
	Expect(unpinned.Ored{}.Error()).To(WithTransform(func(s string) string { return s }, Equal("ored")))

	Expect(func() {
		var busy unpinned.Busy
		_ = busy.Error()
	}).To(Panic())
	Expect(func() { _ = unpinned.Calm{}.Error() }).NotTo(Panic())
}
