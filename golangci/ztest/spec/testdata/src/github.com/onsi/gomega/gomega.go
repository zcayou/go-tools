// Package gomega stands in for Gomega, which is not on a fixture's import path
// since analysistest loads with GOPROXY=off. It sits at Gomega's import path,
// which is what the rules resolve a call against, and declares the part of the
// DSL the fixtures call.
package gomega

type OmegaMatcher interface {
	Match(actual any) (bool, error)
}

type matcher struct{}

func (matcher) Match(actual any) (bool, error) { return true, nil }

type Assertion interface {
	To(matcher OmegaMatcher, description ...any) bool
	ToNot(matcher OmegaMatcher, description ...any) bool
	NotTo(matcher OmegaMatcher, description ...any) bool
	Should(matcher OmegaMatcher, description ...any) bool
	ShouldNot(matcher OmegaMatcher, description ...any) bool
	WithOffset(offset int) Assertion
	Error() Assertion
}

type AsyncAssertion interface {
	Should(matcher OmegaMatcher, description ...any) bool
	ShouldNot(matcher OmegaMatcher, description ...any) bool
	WithTimeout(timeout any) AsyncAssertion
}

type assertion struct{}

func (assertion) To(OmegaMatcher, ...any) bool        { return true }
func (assertion) ToNot(OmegaMatcher, ...any) bool     { return true }
func (assertion) NotTo(OmegaMatcher, ...any) bool     { return true }
func (assertion) Should(OmegaMatcher, ...any) bool    { return true }
func (assertion) ShouldNot(OmegaMatcher, ...any) bool { return true }
func (a assertion) WithOffset(int) Assertion          { return a }
func (a assertion) Error() Assertion                  { return a }
func (a assertion) WithTimeout(any) AsyncAssertion    { return a }

type Gomega interface {
	Expect(actual any, extra ...any) Assertion
	Ω(actual any, extra ...any) Assertion
	ExpectWithOffset(offset int, actual any, extra ...any) Assertion
	Eventually(actual any, args ...any) AsyncAssertion
}

type WithT struct{}

func (*WithT) Expect(actual any, extra ...any) Assertion                       { return assertion{} }
func (*WithT) Ω(actual any, extra ...any) Assertion                            { return assertion{} }
func (*WithT) ExpectWithOffset(offset int, actual any, extra ...any) Assertion { return assertion{} }
func (*WithT) Eventually(actual any, args ...any) AsyncAssertion               { return assertion{} }

func NewWithT(t any) *WithT { return &WithT{} }

func NewGomega(fail func(message string, callerSkip ...int)) Gomega { return &WithT{} }

func RegisterFailHandler(handler func(message string, callerSkip ...int)) {}

func Expect(actual any, extra ...any) Assertion { return assertion{} }

func Ω(actual any, extra ...any) Assertion { return assertion{} }

func ExpectWithOffset(offset int, actual any, extra ...any) Assertion { return assertion{} }

func Eventually(actual any, args ...any) AsyncAssertion { return assertion{} }

func Consistently(actual any, args ...any) AsyncAssertion { return assertion{} }

func HaveOccurred() OmegaMatcher { return matcher{} }

func Succeed() OmegaMatcher { return matcher{} }

func Not(matcher OmegaMatcher) OmegaMatcher { return matcher }

func And(matchers ...OmegaMatcher) OmegaMatcher { return matcher{} }

func Or(matchers ...OmegaMatcher) OmegaMatcher { return matcher{} }

func SatisfyAll(matchers ...OmegaMatcher) OmegaMatcher { return matcher{} }

func SatisfyAny(matchers ...OmegaMatcher) OmegaMatcher { return matcher{} }

func MatchError(expected any, functionErrorDescription ...any) OmegaMatcher { return matcher{} }

func ContainSubstring(substr string, args ...any) OmegaMatcher { return matcher{} }

func HavePrefix(prefix string, args ...any) OmegaMatcher { return matcher{} }

func HaveSuffix(suffix string, args ...any) OmegaMatcher { return matcher{} }

func MatchRegexp(regexp string, args ...any) OmegaMatcher { return matcher{} }

func Equal(expected any) OmegaMatcher { return matcher{} }

func Panic() OmegaMatcher { return matcher{} }

func PanicWith(expected any) OmegaMatcher { return matcher{} }

func BeEquivalentTo(expected any) OmegaMatcher { return matcher{} }

func HaveField(field string, expected any) OmegaMatcher { return matcher{} }

func BeAssignableToTypeOf(expected any) OmegaMatcher { return matcher{} }

func BeNil() OmegaMatcher { return matcher{} }

func EventuallyWithOffset(offset int, actual any, args ...any) AsyncAssertion { return assertion{} }

func ConsistentlyWithOffset(offset int, actual any, args ...any) AsyncAssertion { return assertion{} }

func WithTransform(transform any, matcher OmegaMatcher) OmegaMatcher { return matcher }

func BeTrue() OmegaMatcher { return matcher{} }

func BeFalse() OmegaMatcher { return matcher{} }

func BeTrueBecause(format string, args ...any) OmegaMatcher { return matcher{} }

func BeFalseBecause(format string, args ...any) OmegaMatcher { return matcher{} }

func BeIdenticalTo(expected any) OmegaMatcher { return matcher{} }
