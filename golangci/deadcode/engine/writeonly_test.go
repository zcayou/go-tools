package engine_test

import (
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

var _ = Describe("write-only-field", func() {
	reported := []string{
		"api/api.go:8:2: write-only-field: Handle.Level",
		"api/api.go:9:2: write-only-field: Handle.mode",
		"api/api.go:20:2: write-only-field: Builder.Out",
		"api/api.go:21:2: write-only-field: Builder.step",
		"api/api.go:32:21: write-only-field: Record.Note",
		"box/box.go:6:20: write-only-field: state.outcome",
		"box/box.go:12:27: write-only-field: unboxedState.outcome",
		"counter/counter.go:13:24: write-only-field: addedOnly.hits",
		"counter/counter.go:15:22: write-only-field: blanked.old",
		"counter/counter.go:17:23: write-only-field: deferred.n",
		"kit/kit.go:17:22: write-only-field: Fixture.X",
		"plan/plan.go:5:23: write-only-field: Internal.Depth",
		"plan/plan.go:10:22: write-only-field: Outcome.Code",
		"plan/plan.go:13:21: write-only-field: Detail.Text",
		"store/store.go:11:2: write-only-field: Literal.Name",
		"store/store.go:12:2: write-only-field: Literal.size",
		"store/store.go:16:19: write-only-field: Pair.left",
		"store/store.go:16:25: write-only-field: Pair.right",
		"store/store.go:25:2: write-only-field: Counter.assigned",
		"store/store.go:26:2: write-only-field: Counter.sum",
		"store/store.go:27:2: write-only-field: Counter.hits",
		"store/store.go:28:2: write-only-field: Counter.log",
		"store/store.go:29:2: write-only-field: Counter.total",
		"store/store.go:30:2: write-only-field: Counter.legacy",
		"store/store.go:43:19: write-only-field: Nest.inner",
		"store/store.go:45:19: write-only-field: part.v",
		"store/store.go:52:2: write-only-field: Box.stamp",
		"store/store.go:71:21: write-only-field: local.n",
		"store/store.go:77:20: write-only-field: Gauge.level",
		"store/store.go:83:20: write-only-field: Token.id",
		"store/store.go:92:23: write-only-field: Snapshot.rev",
		"store/store_test.go:14:19: write-only-field: fake.calls",
	}
	writeOnly := func(cfg engine.Config) engine.Config {
		cfg.Dir = fixtureDir("writeonly")
		cfg.Rules = []engine.Rule{engine.RuleWriteOnlyField}
		return cfg
	}

	It("reports a field module code writes and nothing reads", func(ctx SpecContext) {
		// Set only in a keyed or a positional composite literal; only assigned,
		// op-assigned, incremented, appended to itself, or atomically added, swapped
		// or given to a deferred atomic.AddInt64 with the result discarded; written
		// beneath a struct-valued field; a generic struct's field across its
		// instantiations; a function-local type's; a test's; one only an in-package
		// or another package's test reads; one held only as a context value
		// or by an atomic.Pointer, or beneath a value converted to an interface only
		// to call its methods; and, with no api declared, exported ones. A field
		// returned, addressed, called on, written through, read in String or only
		// in dead code; every field of a printed, described, deeply compared,
		// compared, keyed, searched, switched on, converted or unsafely converted
		// struct, each of several sharing one shape; an atomic field loaded
		// or compared and swapped, or added to or swapped for a result the code uses;
		// an embedded field, an anonymous struct's, and a generated file's are not
		// reported.
		Expect(ruleFindings(ctx, writeOnly(engine.Config{Tests: true}), engine.RuleWriteOnlyField)).To(Equal(reported))
	})

	It("says a field nothing reads is never read", func(ctx SpecContext) {
		found, err := engine.Analyze(ctx, writeOnly(engine.Config{Tests: true}))

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(ContainElement(SatisfyAll(
			HaveField("Name", "Literal.Name"),
			HaveField("Detail", "Literal.Name is written and never read; read it or remove it"),
		)))
	})

	It("says a production field only tests read is read only by tests", func(ctx SpecContext) {
		found, err := engine.Analyze(ctx, writeOnly(engine.Config{Tests: true}))

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(ContainElement(SatisfyAll(
			HaveField("Name", "Gauge.level"),
			HaveField("Detail", "Gauge.level is written and read only by tests; read it in production code or remove it"),
		)))
	})

	It("counts a test-facing package's reads as the tests' and the tests' toward its fields", func(ctx SpecContext) {
		cfg := writeOnly(engine.Config{Tests: true, TestFacing: []string{"./kit"}})
		found, err := engine.Analyze(ctx, cfg)

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(ContainElement(SatisfyAll(
			HaveField("Name", "Probe.Depth"),
			HaveField("Detail", "Probe.Depth is written and read only by tests; read it in production code or remove it"),
		)))
		Expect(found).NotTo(ContainElement(HaveField("Name", "Harness.runs")))
		Expect(found).NotTo(ContainElement(HaveField("Name", "Fixture.X")))
	})

	It("leaves the exported fields of the struct types a consumer can hold to it", func(ctx SpecContext) {
		// An exported type of an api package is held whether or not a signature
		// carries it, and so is what an API signature reaches, the methods of what
		// it reaches included. Unexported fields, and an exported type of a package
		// outside the api that nothing a consumer holds reaches, are still judged.
		exposed := []string{
			"api/api.go:8:2: write-only-field: Handle.Level",
			"api/api.go:20:2: write-only-field: Builder.Out",
			"api/api.go:32:21: write-only-field: Record.Note",
			"plan/plan.go:10:22: write-only-field: Outcome.Code",
			"plan/plan.go:13:21: write-only-field: Detail.Text",
		}
		found := ruleFindings(ctx, writeOnly(engine.Config{Tests: true, API: []string{"./api"}}), engine.RuleWriteOnlyField)

		Expect(found).To(Equal(slices.DeleteFunc(slices.Clone(reported), func(finding string) bool {
			return slices.Contains(exposed, finding)
		})))
	})

	It("judges no test file without tests", func(ctx SpecContext) {
		found, err := engine.Analyze(ctx, writeOnly(engine.Config{}))

		Expect(err).NotTo(HaveOccurred())
		Expect(found).NotTo(ContainElement(HaveField("Name", "fake.calls")))
		Expect(found).To(ContainElement(SatisfyAll(
			HaveField("Name", "Gauge.level"),
			HaveField("Detail", "Gauge.level is written and never read; read it or remove it"),
		)))
	})
})
