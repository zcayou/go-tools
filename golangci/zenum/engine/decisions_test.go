package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("token decisions", func() {
	It("reports each decision on a rendered token, and each comparison with a constant naming no member", func() {
		Expect(analyzeFixture("decisions", engine.Config{})).To(Equal([]string{
			// v.String(), directly, through a strings matcher, through a transformation,
			// through an embedding struct, and switched on. Comparing with ""
			// and with another rendering decides nothing.
			"decisions.go:11:16: token-decision: example.com/decisions/kinds.Kind",
			"decisions.go:13:22: token-decision: example.com/decisions/kinds.Kind",
			"decisions.go:15:33: token-decision: example.com/decisions/kinds.Kind",
			// string(v) of a string enum, and fmt.Sprint(v).
			"decisions.go:17:15: token-decision: example.com/decisions/kinds.Mode",
			"decisions.go:19:19: token-decision: example.com/decisions/kinds.Kind",
			"decisions.go:21:16: token-decision: example.com/decisions/kinds.Kind",
			"decisions.go:24:7: token-decision: example.com/decisions/kinds.Kind",
			"decisions.go:25:7: token-decision: example.com/decisions/kinds.Kind",
			// A constant no member holds, on either side and in a case. A member,
			// a re-exported member, and zero where zero is no member are clean.
			"decisions.go:34:7: token-decision: example.com/decisions/kinds.Kind",
			"decisions.go:36:7: token-decision: example.com/decisions/kinds.Kind",
			"decisions.go:40:7: token-decision: example.com/decisions/kinds.Kind",
			// Zero spelled as a number where zero is a member.
			"decisions.go:48:7: token-decision: example.com/decisions/kinds.Level",
			"decisions.go:52:7: token-decision: example.com/decisions/kinds.Mode",
			// A member's value spelled as anything but the member, outside the declaring
			// package.
			"decisions.go:58:7: token-decision: example.com/decisions/kinds.Kind",
			"decisions.go:61:7: token-decision: example.com/decisions/kinds.Kind",
			"decisions.go:63:7: token-decision: example.com/decisions/kinds.Mode",
			"kinds/kinds.go:24:6: string-enum: example.com/decisions/kinds.Mode",
			"kinds/kinds.go:31:2: zero-member: example.com/decisions/kinds.Level",
		}))
	})

	It("says what was decided on", func() {
		found := messages("decisions", engine.Config{})

		Expect(found).To(HaveKeyWithValue("decisions.go:11:16",
			"token-decision: decides on kinds.Kind's rendered token; compare the value with its members instead"))
		Expect(found).To(HaveKeyWithValue("decisions.go:34:7",
			"token-decision: compares kinds.Kind with 7, which no member holds"))
		Expect(found).To(HaveKeyWithValue("decisions.go:48:7",
			"token-decision: compares kinds.Level with zero, which is its member LevelNone; compare with the member by name"))
		Expect(found).To(HaveKeyWithValue("decisions.go:52:7",
			`token-decision: compares kinds.Mode with "nope", which no member holds`))
		Expect(found).To(HaveKeyWithValue("decisions.go:58:7",
			"token-decision: compares kinds.Kind with 2, which is KindB's value; compare with KindB"))
	})
})
