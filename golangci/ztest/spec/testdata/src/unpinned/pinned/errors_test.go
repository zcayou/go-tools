package pinned_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"unpinned/pinned"
)

func TestGomega(t *testing.T) {
	Expect(pinned.Literal{}.Error()).To(Equal("literal"))

	variable := &pinned.Variable{Code: 1}
	Expect(variable.Error()).Should(Equal("variable"))

	parameter(&pinned.Parameter{})

	Expect(pinned.Do(1).(*pinned.Asserted).Error()).To(Equal("asserted"))

	switch err := pinned.Do(2).(type) {
	case *pinned.Switched:
		Expect(err.Error()).To(Equal("switched"))
	}

	Expect(&pinned.Matched{}).To(MatchError("matched"))
	Expect(pinned.Composed{}.Error()).To(And(HavePrefix("comp"), SatisfyAll(Equal("composed"))))
	Expect(pinned.Generic[int]{Value: 1}.Error()).To(Equal("generic"))
	Expect(func() { _ = pinned.Panicking{}.Error() }).To(Panic())
	Expect(func() { (&pinned.PanickingWith{}).Error() }).To(PanicWith("panicking with"))
}

func parameter(err *pinned.Parameter) {
	Expect(err.Error()).To(Equal("parameter"))
}

var _ = DescribeTable("renders",
	func(err *pinned.Tabled, text string) {
		Expect(err.Error()).To(Equal(text))
	},
	Entry(nil, &pinned.Tabled{}, "tabled"),
)

func TestTestify(t *testing.T) {
	assert.Equal(t, "equaled", pinned.Equaled{}.Error())
	assert.Equal(t, pinned.Reversed{}.Error(), "reversed")
	assert.Exactly(t, "exact", pinned.Exact{}.Error())
	assert.EqualError(t, &pinned.EqualedError{}, "equaled error")
	assert.Equalf(t, "formatted", pinned.Formatted{}.Error(), "renders %s", "formatted")
	require.EqualError(t, &pinned.Required{}, "required")
	assert.New(t).Exactly("method", pinned.Method{}.Error())
	require.Panics(t, func() { _ = pinned.Hostile{}.Error() })
	assert.PanicsWithValue(t, "hostile value", func() { _ = pinned.HostileValue{}.Error() })
}
