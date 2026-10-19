package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

var _ = Describe("Following an error through module code", func() {
	It("follows helpers, closed interfaces, error fields and closures, and leaves open interfaces unjudged", func() {
		// Clean, Fetch, Settle and the recursive call draw nothing: a helper
		// passing on a structured argument, an open interface's call, an error
		// field only ever stored with a structured value, and a judged function
		// calling itself, which carries its own finding.
		Expect(analyze("followed", engine.Config{Sentinels: true}, engine.RuleUnstructuredReturn)).To(ConsistOf(
			"followed.go:17: unstructured-return: Open can return an unstructured error: fmt.Errorf in readManifest",
			"followed.go:21: unstructured-return: Wrapped can return an unstructured error: io.EOF",
			"followed.go:42: unstructured-return: Get can return an unstructured error: errors.New in memory.get",
			"followed.go:49: unstructured-return: Close can return an unstructured error: os.File.Close",
			"followed.go:63: unstructured-return: Run can return an unstructured error: os.Remove in run",
			"followed.go:74: unstructured-return: Recursive can return an unstructured error: errors.New in countdown",
			"followed.go:88: unstructured-return: Immediate can return an unstructured error: io.EOF",
		))
	})

	It("resolves what a closure captures per call, through any nesting", func() {
		// CaptureNameable shares capture with CaptureRaw, whose raw argument is its
		// own: a closure's captured parameter is the call's argument.
		Expect(analyze("flow", engine.Config{}, engine.RuleUnstructuredReturn)).To(ContainElements(
			"flow.go:21: unstructured-return: CaptureRaw can return an unstructured error: io.EOF",
			"flow.go:27: unstructured-return: Nested can return an unstructured error: io.ErrUnexpectedEOF",
		))
		Expect(analyze("flow", engine.Config{}, engine.RuleUnstructuredReturn)).NotTo(ContainElement(
			ContainSubstring("CaptureNameable")))
	})

	It("reports a return inside a range-over-func loop where it is, and no other return", func() {
		found := analyze("flow", engine.Config{}, engine.RuleUnstructuredReturn)

		Expect(found).To(ContainElement(
			"flow.go:33: unstructured-return: Ranged can return an unstructured error: errors.New"))
		Expect(found).NotTo(ContainElement(HavePrefix("flow.go:36:")))
	})

	It("follows a wrapping of parameters per call, and weighs a nil operand as nothing", func() {
		// Service.Run waits on a group a callback was handed through the same
		// field. JoinParameter wraps its caller's own error, and Save and Annotated
		// hand a helper wrapping its parameter a nameable one. Checked wraps
		// an error that is nil or nameable once it is not nil. Cleanup joins a nil
		// result with a raw one, and JoinNil a nil with a raw one.
		Expect(analyze("flow", engine.Config{}, engine.RuleUnstructuredReturn)).To(ConsistOf(
			"flow.go:21: unstructured-return: CaptureRaw can return an unstructured error: io.EOF",
			"flow.go:27: unstructured-return: Nested can return an unstructured error: io.ErrUnexpectedEOF",
			"flow.go:33: unstructured-return: Ranged can return an unstructured error: errors.New",
			"flow.go:57: unstructured-return: AnnotatedRaw can return an unstructured error: fmt.Errorf in annotate",
			"flow.go:61: unstructured-return: Cleanup can return an unstructured error: errors.Join",
			"flow.go:64: unstructured-return: JoinNil can return an unstructured error: errors.Join",
		))
	})

	It("reads a named result through the closures deferred to define or amend it", func() {
		// Attributed's raw refusal is replaced by what the closure deferred first
		// on entry stores, as an attributing adapter's is; AttributedWhen defers
		// it on one path only and AttributedIf assigns on one path only, so each
		// still returns the raw refusal. The close-joins copied and closed defer read
		// what the returns gave, not the raw open failure stored before them, and no
		// deferred call of theirs recovers, so no recovered panic resumes them
		// with that store. recovered's deferred call does recover.
		Expect(analyze("deferred", engine.Config{}, engine.RuleUnstructuredReturn)).To(ConsistOf(
			"deferred.go:45: unstructured-return: AttributedWhen can return an unstructured error: errors.New",
			"deferred.go:54: unstructured-return: AttributedIf can return an unstructured error: errors.New",
			"deferred.go:87: unstructured-return: Recovered can return an unstructured error: os.Open in recovered",
		))
	})

	It("does not follow a cause slot, whose value was judged where it was made", func() {
		// encode stores a raw cause into one Error, Lookup a nameable one into
		// another. Cause returns a loaded cause and Rewrapped wraps one alone,
		// and both draw nothing; Joined joins one with a raw error, which the cause
		// does not structure. Released joins two errors that can be a cause,
		// nameable or nil, which are structured.
		Expect(analyze("causes", engine.Config{}, engine.RuleUnstructuredReturn)).To(ConsistOf(
			"causes.go:30: unstructured-return: Joined can return an unstructured error: errors.Join",
		))
	})
})
