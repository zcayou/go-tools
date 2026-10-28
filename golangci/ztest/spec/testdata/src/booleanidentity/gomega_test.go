package booleanidentity_test

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"booleanidentity"
)

func TestReported(t *testing.T) {
	err := booleanidentity.Do()

	Expect(errors.Is(err, booleanidentity.ErrMissing)).To(BeTrue())                       // want `^boolean-identity: asserts whether errors\.Is found its target, so a failure shows no error; use MatchError\(target\)$`
	Expect(errors.Is(err, booleanidentity.ErrClosed)).To(BeFalse())                       // want `use NotTo\(MatchError\(target\)\)$`
	Expect(errors.Is(err, booleanidentity.ErrMissing)).To(BeTrueBecause("it is missing")) // want `use MatchError\(target\)$`
	Expect(errors.Is(err, booleanidentity.ErrClosed)).To(BeFalseBecause("it is open"))    // want `use NotTo\(MatchError\(target\)\)$`
	Expect(errors.Is(err, booleanidentity.ErrMissing)).To(Equal(true))                    // want `use MatchError\(target\)$`
	Expect(errors.Is(err, booleanidentity.ErrClosed)).To(BeEquivalentTo(false))           // want `use NotTo\(MatchError\(target\)\)$`
	Expect(errors.Is(err, booleanidentity.ErrMissing)).Should(BeIdenticalTo(true))        // want `use MatchError\(target\)$`

	want := len(t.Name()) > 0
	Expect(errors.Is(err, booleanidentity.ErrMissing)).To(Equal(want)) // want `use MatchError\(target\) or NotTo\(MatchError\(target\)\)$`

	Expect(errors.Is(err, booleanidentity.ErrClosed)).NotTo(BeTrue())                 // want `use NotTo\(MatchError\(target\)\)$`
	Expect(errors.Is(err, booleanidentity.ErrClosed)).To(Not(BeTrue()))               // want `use NotTo\(MatchError\(target\)\)$`
	Expect(errors.Is(err, booleanidentity.ErrMissing)).To(And(BeTrue()))              // want `use MatchError\(target\)$`
	Expect(errors.Is(err, booleanidentity.ErrMissing)).To(Or(BeTrue()))               // want `use MatchError\(target\)$`
	Expect(errors.Is(err, booleanidentity.ErrMissing)).To(SatisfyAll(BeTrue()))       // want `use MatchError\(target\)$`
	Expect(errors.Is(err, booleanidentity.ErrMissing)).To(SatisfyAny(Not(BeFalse()))) // want `use MatchError\(target\)$`

	found := errors.Is(err, booleanidentity.ErrMissing)
	Expect(found).To(BeTrue())                                                                                   // want `use MatchError\(target\)$`
	Expect(!errors.Is(err, booleanidentity.ErrClosed)).To(BeTrue())                                              // want `use NotTo\(MatchError\(target\)\)$`
	Expect((errors.Is(err, booleanidentity.ErrMissing))).To(BeTrue())                                            // want `use MatchError\(target\)$`
	Expect(errors.Is(err, booleanidentity.ErrMissing) || errors.Is(err, booleanidentity.ErrClosed)).To(BeTrue()) // want `errors\.Is found its target, .*; use MatchError\(target\)$`

	g := NewWithT(t)
	g.Expect(errors.Is(err, booleanidentity.ErrMissing)).To(BeTrue()) // want `use MatchError\(target\)$`
}

func TestAdmitted(t *testing.T) {
	err := booleanidentity.Do()

	// Gomega has no matcher performing errors.As, so an As boolean is the only
	// way to assert one there.
	var target *booleanidentity.CodeError
	Expect(errors.As(err, &target)).To(BeTrue())
	coded, ok := errors.AsType[*booleanidentity.CodeError](err)
	Expect(ok).To(BeTrue())
	Expect(coded.Code).To(Equal(1))
	Expect(errors.Is(err, booleanidentity.ErrMissing) && errors.As(err, &target)).To(BeTrue())

	if errors.Is(err, booleanidentity.ErrMissing) {
		Expect(err).To(MatchError(booleanidentity.ErrMissing))
	}
	Expect(err).To(WithTransform(func(err error) bool {
		return errors.Is(err, booleanidentity.ErrMissing)
	}, BeTrue()))

	mixed := errors.Is(err, booleanidentity.ErrMissing)
	mixed = len(t.Name()) > 0
	Expect(mixed).To(BeTrue())

	Expect(errors.Is(err, booleanidentity.ErrMissing)).To(Equal(1))
	Expect(BeTrue()).NotTo(BeNil())
}
