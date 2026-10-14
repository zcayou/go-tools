package enum_test

import (
	"go/constant"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/enum"
)

var _ = Describe("Tokens", func() {
	var pkg enum.Package
	BeforeEach(func() { pkg = fixture() })

	// tokensOf renders every member value's token, or "<none>" for a value
	// that renders as none.
	tokensOf := func(name string) []string {
		e := declared(pkg, name)
		tokens := e.Tokens()
		Expect(tokens.Readable).To(BeTrue(), "%s is unreadable", name)
		values := e.Values()
		out := make([]string, 0, len(values))
		for _, value := range values {
			token, ok := tokens.Token(value)
			if !ok {
				token = "<none>"
			}
			out = append(out, token)
		}
		return out
	}

	DescribeTable("reads each readable form",
		func(name string, want []string) {
			Expect(tokensOf(name)).To(Equal(want))
		},
		Entry("a switch on the receiver", "Switch", []string{"a", "b", "c"}),
		Entry("a guarded table", "Table", []string{"first", "second"}),
		Entry("a table inside a guard, at an offset", "Offset", []string{"first", "second"}),
		Entry("a map keyed by member", "Mapped", []string{"one", "two"}),
		Entry("stringer's single run", "Single", []string{"SingleA", "SingleB", "SingleC"}),
		Entry("stringer's runs", "Runs", []string{"RunsA", "RunsB", "RunsC", "RunsD", "RunsE"}),
		Entry("stringer's map", "Sparse", []string{
			"Sparse1", "Sparse3", "Sparse5", "Sparse7", "Sparse9", "Sparse11", "Sparse13", "Sparse15",
			"Sparse17", "Sparse19", "Sparse21", "Sparse23",
		}),
		Entry("a string enum's values", "Mode", []string{"read", "write"}),
		Entry("an if chain", "Chain", []string{"chain-a", "chain-b", "<none>"}),
		Entry("a switch with no tag", "Tagless", []string{"tagless-a", "tagless-b"}),
		Entry("a case falling through into the next", "Through", []string{"through", "through", "<none>"}),
		Entry("an array keyed by member", "Keyed", []string{"keyed-a", "keyed-b"}),
		Entry("locals: a constant, a variable each arm assigns, and a decrement", "Local",
			[]string{"local-a", "local-b"}),
		Entry("a name an equality picks out beside a bounds check", "Special", []string{"special-a", "special-b"}),
		Entry("an offset an unsigned value wraps past", "Wrapped", []string{"high", "top", "zero"}),
		Entry("one string returned before anything tells values apart", "Constant",
			[]string{"constant", "constant"}),
	)

	DescribeTable("gives no token to a member that reaches the fallback",
		func(name string, want []string) {
			Expect(tokensOf(name)).To(Equal(want))
		},
		Entry("the code after a switch, beside empty and shared tokens", "Ambiguous",
			[]string{"same", "same", "", "<none>"}),
		Entry("a hand-written lookup that finds nothing", "CommaOk", []string{"comma-ok-a", "<none>"}),
		Entry("a map's empty string for a key it lacks", "Holey", []string{"holey-a", "<none>"}),
		Entry("a panic in the default arm", "Panicky", []string{"panicky-a", "<none>"}),
		Entry("a bounds check against how many entries a map holds", "MapLen", []string{"map-len-one", "<none>"}),
	)

	DescribeTable("cannot read a String that renders by any other means",
		func(name string) {
			tokens := declared(pkg, name).Tokens()

			Expect(tokens.Method).NotTo(BeNil())
			Expect(tokens.Readable).To(BeFalse())
			_, ok := tokens.Token(constant.MakeInt64(1))
			Expect(ok).To(BeFalse())
		},
		Entry("one that formats", "Formatted"),
		Entry("one that names no member", "Unnamed"),
		Entry("one that computes a member's name in another method", "Delegating"),
	)

	It("reads nothing of an ordinal enum with no name", func() {
		tokens := declared(pkg, "Nameless").Tokens()

		Expect(tokens.Method).To(BeNil())
		Expect(tokens.Readable).To(BeFalse())
	})
})
