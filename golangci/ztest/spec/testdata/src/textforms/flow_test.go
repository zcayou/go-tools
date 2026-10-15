package textforms_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"textforms"
)

func TestTextThroughVariables(t *testing.T) {
	err := textforms.Do()

	msg := err.Error()
	Expect(msg).To(Equal("boom")) // want `^text-assertion`
	lower := strings.ToLower(msg)
	Expect(lower).To(ContainSubstring("bo"))                 // want `^text-assertion`
	Expect(strings.TrimSpace(err.Error())).To(Equal("boom")) // want `^text-assertion`
	Expect(msg[1:]).To(Equal("oom"))                         // want `^text-assertion`
	Expect("failed: " + msg).To(Equal("failed: boom"))       // want `^text-assertion`
	Expect(err).To(MatchError(msg))                          // want `^text-assertion`
	Expect(msg).NotTo(ContainSubstring("secret"))

	rewritten := err.Error()
	rewritten = "elsewhere"
	Expect(rewritten).To(Equal("elsewhere"))

	var split []string
	Expect(strings.Split(msg, ":")).To(Equal(split))
}

func TestWithTransform(t *testing.T) {
	err := textforms.Do()

	Expect(err.Error()).To(WithTransform(strings.ToLower, Equal("boom"))) // want `^text-assertion`
	Expect(err.Error()).To(WithTransform(strings.ToLower, Not(ContainSubstring("secret"))))
	Expect(err.Error()).To(WithTransform(func(s string) int { return len(s) }, Equal(4)))
	Expect(err).To(MatchError(WithTransform(strings.ToLower, ContainSubstring("boom")))) // want `^text-assertion`
	Expect(err).To(MatchError(WithTransform(strings.ToLower, Not(ContainSubstring("secret")))))
	Expect(err).To(MatchError(WithTransform(func(s string) int { return len(s) }, Equal(4))))
	Expect(err).To(HaveField("Error()", WithTransform(func(s string) int { return len(s) }, Equal(4))))
}

func TestAsync(t *testing.T) {
	err := textforms.Do()
	name := "it"

	EventuallyWithOffset(1, err.Error()).Should(Equal("boom"))   // want `^text-assertion`
	ConsistentlyWithOffset(1, err.Error()).Should(Equal("boom")) // want `^text-assertion`

	ctx := t.Context()
	Eventually(ctx, err.Error()).Should(Equal("boom")) // want `^text-assertion`
	cancellable, cancel := context.WithCancel(ctx)
	defer cancel()
	Consistently(cancellable, err.Error()).Should(Equal("boom")) // want `^text-assertion`
	Eventually(t.Context(), err.Error()).Should(Equal("boom"))   // want `^text-assertion`
	Eventually(name, err.Error()).Should(Equal("boom"))
}

func inSpec(ctx ginkgo.SpecContext, err error) {
	Eventually(ctx, err.Error()).Should(Equal("boom")) // want `^text-assertion`
}

func throughTypes(g types.Gomega, err error) {
	g.Expect(err.Error()).To(Equal("boom")) // want `^text-assertion`
}

func TestCauseFlow(t *testing.T) {
	err := fmt.Errorf("wrapping: %w", textforms.Do())

	cause := errors.Unwrap(err)
	cause = err
	Expect(cause).To(MatchError("boom")) // want `^text-assertion`

	if c := errors.Unwrap(err); c != nil {
		Expect(c).To(MatchError("boom"))
	}
	{
		c := err
		Expect(c).To(MatchError("boom")) // want `^text-assertion`
	}

	text := errors.Unwrap(err).Error()
	Expect(text).To(Equal("boom"))
}

func TestBuiltError(t *testing.T) {
	err := textforms.Do()

	Expect(err).To(MatchError(errors.New("boom")))           // want `^text-assertion: matches an error built in the spec, which Gomega compares by its text; match the error's identity or type instead$`
	Expect(err).To(MatchError(fmt.Errorf("doing %s", "it"))) // want `^text-assertion: matches an error built in the spec`
	Expect(err).NotTo(MatchError(errors.New("secret")))
	Expect(errors.Unwrap(fmt.Errorf("w: %w", err))).To(MatchError(errors.New("boom")))
}
