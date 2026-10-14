package engine_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("text-decision", func() {
	It("reports each decision on an error's text, and no comparison that decides nothing", func() {
		// empty compares with "", both compares two subjects, ordered sorts by text
		// and code decides on a field that is not a string: none decides on text.
		Expect(decisions("decisions")).To(ConsistOf(
			"decisions.go:26 contains",
			"decisions.go:28 prefix",
			"decisions.go:32 matches",
			"decisions.go:34 equal",
			"decisions.go:36 notEqual",
			"decisions.go:40 switched",
			"decisions.go:46 sprint",
			"decisions.go:48 sprintf",
			"decisions.go:50 field",
			"decisions.go:52 embedded",
			"decisions.go:54 direct",
			"decisions.go:56 transformed",
			"decisions.go:63 local",
		))
	})

	It("says what to branch on instead", func() {
		Expect(analyze("decisions", engine.Config{}, engine.RuleTextDecision)).To(ContainElements(
			"decisions.go:26: text-decision: decides on an error's text; branch on its type or fields instead",
			"decisions.go:50: text-decision: decides on the text of decisions.Failure.Reason; "+
				"give the field an enum type and branch on its members",
			"decisions.go:52: text-decision: decides on the text of decisions.detail.Kind; "+
				"give the field an enum type and branch on its members",
		))
	})
})

// decisions renders each text decision in a fixture as its file, line
// and the function making it.
func decisions(fixture string) []string {
	GinkgoHelper()

	found, err := run(fixture, engine.Config{})
	Expect(err).NotTo(HaveOccurred())
	var rendered []string
	for _, finding := range found {
		if finding.Rule == engine.RuleTextDecision {
			rendered = append(rendered, fmt.Sprintf("%s.go:%d %s", fixture, finding.Pos.Line, finding.Subject))
		}
	}
	return rendered
}
