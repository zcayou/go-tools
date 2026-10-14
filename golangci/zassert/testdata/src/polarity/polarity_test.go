package polarity_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"polarity"
)

func TestPositive(t *testing.T) {
	err := polarity.Do()

	Expect(err).To(HaveOccurred())                     // want `^unidentified-error: HaveOccurred says only that some error occurred`
	Expect(err).Should(HaveOccurred())                 // want `^unidentified-error: HaveOccurred`
	Ω(err).Should(HaveOccurred())                      // want `^unidentified-error: HaveOccurred`
	ExpectWithOffset(1, err).To(HaveOccurred())        // want `^unidentified-error: HaveOccurred`
	Eventually(polarity.Do).Should(HaveOccurred())     // want `^unidentified-error: HaveOccurred`
	Expect(err).WithOffset(1).To(HaveOccurred())       // want `^unidentified-error: HaveOccurred`
	Expect(polarity.Pair()).Error().To(HaveOccurred()) // want `^unidentified-error: HaveOccurred`
	Expect(polarity.Do()).NotTo(Succeed())             // want `^unidentified-error: a negated Succeed says only`
	Expect(polarity.Do()).ToNot(Succeed())             // want `^unidentified-error: a negated Succeed`
	Consistently(polarity.Do).ShouldNot(Succeed())     // want `^unidentified-error: a negated Succeed`
}

func TestNegative(t *testing.T) {
	err := polarity.Do()

	Expect(err).NotTo(HaveOccurred())
	Expect(err).ToNot(HaveOccurred())
	Expect(err).ShouldNot(HaveOccurred())
	Expect(polarity.Do()).To(Succeed())
	Expect(polarity.Do()).Should(Succeed())
	Expect(err).To(MatchError(polarity.ErrBoom))
}

func TestNot(t *testing.T) {
	err := polarity.Do()

	Expect(err).To(Not(HaveOccurred()))
	Expect(err).NotTo(Not(HaveOccurred()))   // want `^unidentified-error: HaveOccurred`
	Expect(err).To(Not(Not(HaveOccurred()))) // want `^unidentified-error: HaveOccurred`
	Expect(polarity.Do()).To(Not(Succeed())) // want `^unidentified-error: a negated Succeed`
	Expect(polarity.Do()).NotTo(Not(Succeed()))
}

func TestComposed(t *testing.T) {
	err := polarity.Do()

	Expect(err).To(And(HaveOccurred(), MatchError(polarity.ErrBoom))) // want `^unidentified-error: HaveOccurred`
	Expect(err).To(SatisfyAll(Not(Not(HaveOccurred())), BeNil()))     // want `^unidentified-error: HaveOccurred`
	Expect(err).NotTo(SatisfyAny(HaveOccurred(), BeNil())) // want `^unidentified-error: a negated BeNil`
	Expect(err).To(Or(Not(HaveOccurred()), BeNil()))
	Expect(polarity.Do()).NotTo(Or(Succeed(), BeNil())) // want `^unidentified-error: a negated Succeed` `^unidentified-error: a negated BeNil`
}

func TestGomegaValues(t *testing.T) {
	err := polarity.Do()

	g := NewWithT(t)
	g.Expect(err).To(HaveOccurred()) // want `^unidentified-error: HaveOccurred`
	g.Expect(err).NotTo(HaveOccurred())
	NewWithT(t).Expect(err).To(HaveOccurred()) // want `^unidentified-error: HaveOccurred`

	Eventually(func(g Gomega) {
		g.Expect(polarity.Do()).To(HaveOccurred()) // want `^unidentified-error: HaveOccurred`
	}).Should(Succeed())
}

// entry stands in for a table entry, which hands a matcher on to whatever
// asserts with it.
func entry(matcher OmegaMatcher) {}

func TestTableEntries(t *testing.T) {
	entry(HaveOccurred()) // want `^unidentified-error: HaveOccurred`
	entry(Not(HaveOccurred()))
	entry(Not(Succeed())) // want `^unidentified-error: a negated Succeed`
	entry(Succeed())
}
