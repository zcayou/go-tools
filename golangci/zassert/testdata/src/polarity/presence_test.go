package polarity_test

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"polarity"
)

func TestPresence(t *testing.T) {
	var err error = polarity.Do()

	Expect(err).NotTo(BeNil())             // want `^unidentified-error: a negated BeNil of an error says only that some error occurred`
	Expect(err).ToNot(BeNil())             // want `^unidentified-error: a negated BeNil`
	Expect(err).ShouldNot(BeNil())         // want `^unidentified-error: a negated BeNil`
	Expect(err).To(Not(BeNil()))           // want `^unidentified-error: a negated BeNil`
	Expect(errors.New("x")).NotTo(BeNil()) // want `^unidentified-error: a negated BeNil`
	Expect(err).To(BeNil())
	Expect(err).NotTo(Not(BeNil()))
	Expect(err).NotTo(WithTransform(errors.Unwrap, BeNil()))

	got := polarity.Do()
	Expect(got).NotTo(BeNil()) // want `^unidentified-error: a negated BeNil`

	_, pair := polarity.Pair()
	Expect(pair).NotTo(BeNil()) // want `^unidentified-error: a negated BeNil`

	var failure *polarity.Failure
	Expect(failure).NotTo(BeNil()) // want `^unidentified-error: a negated BeNil`

	value := 1
	Expect(&value).NotTo(BeNil())
}
