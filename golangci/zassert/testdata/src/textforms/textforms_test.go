package textforms_test

import (
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	"textforms"
)

func TestMatchErrorText(t *testing.T) {
	err := textforms.Do()
	name := "it"

	Expect(err).To(MatchError("boom"))                                // want `^text-assertion: asserts an error's text, which only the spec of the source declaring its Error method pins; assert the error's type and fields instead$`
	Expect(err).To(MatchError("doing " + name))                       // want `^text-assertion`
	Expect(err).To(MatchError(name + ": " + name))                    // want `^text-assertion`
	Expect(err).To(MatchError(fmt.Sprintf("doing %s", name)))         // want `^text-assertion`
	Expect(err).To(MatchError(textforms.ErrBoom.Error()))             // want `^text-assertion`
	Expect(err).To(MatchError(ContainSubstring("boom")))              // want `^text-assertion`
	Expect(err).To(MatchError(HavePrefix("bo")))                      // want `^text-assertion`
	Expect(err).To(MatchError(HaveSuffix("om")))                      // want `^text-assertion`
	Expect(err).To(MatchError(MatchRegexp("^bo+m$")))                 // want `^text-assertion`
	Expect(err).To(MatchError(Equal("boom")))                         // want `^text-assertion`
	Expect(err).To(MatchError(BeEquivalentTo("boom")))                // want `^text-assertion`
	Expect(err).To(MatchError(And(HavePrefix("b"), HaveSuffix("m")))) // want `^text-assertion`
	Expect(err).To(HaveField("Error()", Equal("boom")))               // want `^text-assertion`
	NewWithT(t).Expect(err).To(MatchError("boom"))                    // want `^text-assertion`
}

func TestActualText(t *testing.T) {
	err := textforms.Do()
	var declared error = textforms.Do()
	made := fmt.Errorf("wrapping: %w", err)

	Expect(err.Error()).To(Equal("boom"))                                         // want `^text-assertion`
	Expect(err.Error()).Should(ContainSubstring("boom"))                          // want `^text-assertion`
	Ω(textforms.Do().Error()).To(HavePrefix("bo"))                                // want `^text-assertion`
	Expect(fmt.Sprint(declared)).To(Equal("boom"))                                // want `^text-assertion`
	Expect(fmt.Sprintf("%v", made)).To(Equal("boom"))                             // want `^text-assertion`
	Expect(err.Error()).To(And(HavePrefix("b"), Not(ContainSubstring("secret")))) // want `^text-assertion`

	g := NewWithT(t)
	g.Expect(err.Error()).To(Equal("boom")) // want `^text-assertion`
}

func TestIdentity(t *testing.T) {
	err := textforms.Do()

	Expect(err).To(MatchError(textforms.ErrBoom))
	Expect(err).To(MatchError(func(err error) bool { return true }, "is boom"))
	Expect(textforms.Pair()).Error().To(MatchError(textforms.ErrBoom))
	Expect(textforms.Pair()).Error().To(Equal(textforms.ErrBoom))
}

// TestUndecided holds what is not shown to be text: a string variable no
// write makes an error's text, given to MatchError, and the rendering of a type
// that declares no constants, which is no enum.
func TestUndecided(t *testing.T) {
	err := textforms.Do()
	var message string
	var kind textforms.Kind

	Expect(err).To(MatchError(message))
	Expect(fmt.Sprint(kind)).To(Equal("running"))
	Expect(kind.String()).To(Equal("running"))
}

// TestTyped holds text only types show: an error from a multi-value call,
// rendered by fmt.Sprint.
func TestTyped(t *testing.T) {
	_, err := textforms.Pair()

	Expect(fmt.Sprint(err)).To(Equal("boom")) // want `^text-assertion: asserts an error's text`
}

func TestNegated(t *testing.T) {
	err := textforms.Do()

	Expect(err).NotTo(MatchError("secret"))
	Expect(err).To(Not(MatchError("secret")))
	Expect(err).To(MatchError(Not(ContainSubstring("secret"))))
	Expect(err.Error()).NotTo(ContainSubstring("secret"))
	Expect(err.Error()).To(Not(ContainSubstring("secret")))
	Expect(err).ShouldNot(HaveField("Error()", ContainSubstring("secret")))
	Expect(err).To(HaveField("Error()", Not(ContainSubstring("secret"))))
}

func TestCause(t *testing.T) {
	err := fmt.Errorf("wrapping: %w", textforms.Do())

	Expect(errors.Unwrap(err)).To(MatchError("boom"))
	Expect(errors.Unwrap(err).Error()).To(Equal("boom"))
	Expect(fmt.Sprint(errors.Unwrap(err))).To(Equal("boom"))

	cause := errors.Unwrap(err)
	Expect(cause).To(MatchError("boom"))
	Expect(cause.Error()).To(Equal("boom"))

	func() {
		Expect(cause).To(HaveField("Error()", Equal("boom")))
	}()

	var assigned error
	assigned = errors.Unwrap(err)
	Expect(assigned).To(MatchError(ContainSubstring("boom")))
}

func TestCauseElsewhere(t *testing.T) {
	err := fmt.Errorf("wrapping: %w", textforms.Do())

	var cause error
	func() { cause = errors.Unwrap(err) }()
	Expect(cause).To(MatchError("boom")) // want `^text-assertion`

	shadowed := errors.Unwrap(err)
	func() {
		shadowed := err
		Expect(shadowed).To(MatchError("boom")) // want `^text-assertion`
	}()
	_ = shadowed
}

// entry stands in for a table entry, which hands a matcher on to whatever
// asserts with it.
func entry(matcher OmegaMatcher) {}

func TestTableEntries(t *testing.T) {
	entry(MatchError("boom")) // want `^text-assertion`
	entry(Not(MatchError("secret")))
}
