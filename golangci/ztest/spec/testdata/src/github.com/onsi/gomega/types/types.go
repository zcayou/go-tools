// Package types stands in for Gomega's types package, which declares
// the Gomega interface the root package aliases.
package types

type GomegaMatcher interface {
	Match(actual any) (bool, error)
}

type Assertion interface {
	To(matcher GomegaMatcher, description ...any) bool
	NotTo(matcher GomegaMatcher, description ...any) bool
}

type Gomega interface {
	Expect(actual any, extra ...any) Assertion
}
