// Package gomega stands in for Gomega, which the fixture module reaches
// through a replace directive rather than the module cache. It declares
// the part of the DSL the fixture calls.
package gomega

type OmegaMatcher interface {
	Match(actual any) (bool, error)
}

type matcher struct{}

func (matcher) Match(actual any) (bool, error) { return true, nil }

type Assertion struct{}

func (Assertion) To(matcher OmegaMatcher, description ...any) bool { return true }

func Expect(actual any, extra ...any) Assertion { return Assertion{} }

func HaveOccurred() OmegaMatcher { return matcher{} }
