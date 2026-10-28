package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("test files", func() {
	tests := engine.Config{Tests: true}

	// kinds' own tests, in-package and external, are inside it: a conversion into
	// Kind, one out of it and an ordering are clean there. secret is printed only
	// by a test, and stays invisible, so nothing reports it unnamed.
	It("judges each test file as the code beside it", func() {
		Expect(analyzeFixture("testfiles", tests)).To(Equal([]string{
			// Inside, a bound by the highest member, and the printer rules, which hold
			// everywhere.
			"kinds/kinds_internal_test.go:13:8: member-bound: example.com/testfiles/kinds.Kind",
			"kinds/kinds_internal_test.go:14:15: numeric-verb: example.com/testfiles/kinds.Kind",
			"kinds/kinds_test.go:17:14: string-bypass: example.com/testfiles/kinds.Kind",
			"kinds/kinds_test.go:18:15: numeric-verb: example.com/testfiles/kinds.Kind",
			// A conversion to a type parameter only a test instantiates with Kind,
			// reported where the generic body writes it.
			"user/user.go:10:40: forged-conversion: example.com/testfiles/kinds.Kind",
			// Outside, every use-site rule: a decision on a token, a member's value
			// spelled as a number, a forged and a laundered conversion, a numeric verb,
			// a bypassed String, and two orderings, the second by the highest member.
			"user/user_test.go:22:16: token-decision: example.com/testfiles/kinds.Kind",
			"user/user_test.go:24:7: token-decision: example.com/testfiles/kinds.Kind",
			"user/user_test.go:26:6: forged-conversion: example.com/testfiles/kinds.Kind",
			"user/user_test.go:27:6: laundered-conversion: example.com/testfiles/kinds.Kind",
			"user/user_test.go:28:19: numeric-verb: example.com/testfiles/kinds.Kind",
			"user/user_test.go:29:14: string-bypass: example.com/testfiles/kinds.Kind",
			"user/user_test.go:30:8: ordered-comparison: example.com/testfiles/kinds.Kind",
			"user/user_test.go:31:8: ordered-comparison: example.com/testfiles/kinds.Kind",
			// Printers a test declares: a function, and an implementation that makes
			// user.Logger one to the test, and to none of user's own code.
			"user/user_test.go:32:13: numeric-verb: example.com/testfiles/kinds.Kind",
			"user/user_test.go:35:15: numeric-verb: example.com/testfiles/kinds.Kind",
		}))
	})

	It("judges no test file when told not to", func() {
		Expect(analyzeFixture("testfiles", engine.Config{})).To(BeNil())
	})

	It("counts a test file's ordering as one ordered-enums suppresses, while it judges tests", func() {
		ordered := []string{"example.com/testfiles/kinds.Kind"}

		Expect(analyzeFixture("testfiles", engine.Config{Tests: true, OrderedEnums: ordered})).NotTo(ContainElement(
			Or(ContainSubstring(": ordered-comparison: "), ContainSubstring(": member-bound: "))))
		Expect(analyzeError("testfiles", engine.Config{OrderedEnums: ordered})).To(MatchError(&engine.ExceptionError{
			Setting: engine.SettingOrderedEnums, Fault: engine.FaultNotOrdered, Type: "example.com/testfiles/kinds.Kind",
		}))
	})
})
