package engine_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

// Each fixture under testdata is its own module holding one program that uses
// nothing but the standard library, so a run needs no network and the analysis
// sees exactly the program the fixture describes.
var _ = Describe("Analyze", func() {
	DescribeTable("interface participation",
		func(ctx SpecContext, fixture string, expected []string) {
			Expect(analyzeFixture(ctx, fixture)).To(Equal(expected))
		},

		// The cases the interface-bind scan exists for. Each of these reports its
		// methods when the scan credits nothing, so an empty expectation
		// is the assertion.
		Entry("credits a handler that only a library invokes",
			"libraryhandler", []string(nil)),
		Entry("credits methods a library discovers by assertion",
			"unwrap", []string(nil)),
		Entry("credits a method whose only evidence is a dependency's interface constraint",
			"constraint", []string(nil)),
		Entry("credits a method the standard library finds through reflect.TypeAssert",
			"marshaler", []string(nil)),

		// A locally declared interface confers a use only while its own method
		// is used, so a dead interface method cannot launder its implementation into
		// looking live. The fixture performs a real conversion, so the rule has
		// to refuse the credit rather than never be offered one.
		Entry("does not let a dead interface method launder its implementation",
			"laundering", []string{
				"main.go:8:2: unused interface method: Reporter.Report",
				"main.go:13:13: unused reflection-live exported method: Sink.Report",
			}),
		Entry("does not let an alias to a local interface launder its implementation",
			"alias", []string{
				"main.go:6:2: unused interface method: reporter.Report",
				"main.go:15:13: unused reflection-live exported method: Sink.Report",
			}),

		// A reflect-typed container: constructors registered as any, invoked through
		// reflect, products surfaced behind interfaces by an assertion
		// in a dependency body SSA never builds. The derived-type closure over
		// the registered constructors' signatures is the only materialization
		// evidence; the container's generic-body assertion and the enrollment
		// constraint credit only once their type parameters are substituted under
		// the program's instantiations; and participation suppresses every method
		// verdict at once. What stays reported: Orphan, which no interface declares
		// (reflection keeps its receiver alive, and that is the whole of its
		// liveness); Mismatch.Apply, whose signature the substituted assertion
		// refuses; and the never-enrolled Shadow with everything it declares.
		Entry("credits methods a reflect-typed container invokes behind interfaces",
			"reflectdi/app", []string{
				"provider/provider.go:23:15: unreachable func: Shadow.Configure",
				"provider/provider.go:29:15: unreachable func: Shadow.ResolveInput",
				"provider/provider.go:21:6: unused exported type: Shadow",
				"facade/facade.go:38:22: unused reflection-live exported method: Runtime.Orphan",
				"meta/meta.go:38:17: unused reflection-live exported method: Mismatch.Apply",
				"provider/provider.go:23:15: unused exported method: Shadow.Configure",
				"provider/provider.go:29:15: unused exported method: Shadow.ResolveInput",
			}),

		// A type whose only materialization is the descriptor reflect.TypeFor
		// conjures, resolved transitively through a generic helper. The same-shaped
		// control no TypeFor names stays reported.
		Entry("credits methods whose receiver only reflect.TypeFor materializes",
			"reflecttype", []string{
				"main.go:22:15: unreachable func: shadow.greet",
			}),

		// Interfaces selected only through identically re-declared adapters: a value
		// conversion, a constraint satisfaction, and an opposite-direction closure
		// pair each carry use back to the declaration behind them, and the credit
		// then flows on to the in-module implementations. The controls hold the line:
		// an identical interface flowing nowhere keeps its finding, and one reaching
		// an adapter only through a type assertion gets no credit.
		Entry("propagates use through identically re-declared adapter interfaces",
			"adapter", []string{
				"profiles/profiles.go:47:2: unused interface method: Ignored.List",
				"profiles/profiles.go:53:2: unused interface method: Watched.List",
			}),

		// The control: participation in nothing is still reported, so a passing suite
		// above is not simply a linter that reports nothing.
		Entry("still reports a method bound to nothing",
			"control", []string{
				"main.go:8:14: unused reflection-live exported method: Gauge.Read",
			}),

		// Structural satisfaction is not participation. fmt asserts Stringer
		// and error in its own source and every program imports fmt, so weighing
		// an assertion against the types the program materializes is what keeps
		// a method of those names reportable at all.
		Entry("reports methods that only satisfy an interface structurally",
			"structural", []string{
				"main.go:10:14: unreachable func: Ghost.String",
				"main.go:11:14: unreachable func: Ghost.Error",
				"main.go:20:15: unreachable func: Bucket.Push",
				"main.go:8:6: unused exported type: Ghost",
				"main.go:18:6: unused exported type: Bucket",
				"main.go:10:14: unused exported method: Ghost.String",
				"main.go:11:14: unused exported method: Ghost.Error",
				"main.go:20:15: unused exported method: Bucket.Push",
			}),
	)

	DescribeTable("what counts as a reference",
		func(ctx SpecContext, fixture string, expected []string) {
			Expect(analyzeFixture(ctx, fixture)).To(Equal(expected))
		},

		// One declaration draws both verdicts, which is the property that makes
		// uniq-by-line lossy. The orphan package is imported by nothing,
		// and is reported all the same.
		Entry("reports a function that is both unreachable and unreferenced",
			"unreachable", []string{
				"main.go:10:6: unreachable func: Dead",
				"main.go:13:6: unreachable func: helper",
				"orphan/orphan.go:5:6: unreachable func: stranded",
				"main.go:10:6: unused exported func: Dead",
			}),

		// A blank var neutralizes only an inert assertion. A computed value runs,
		// and so does a function literal's body, so both count as uses of everything
		// they name.
		Entry("neutralizes an inert blank assertion and nothing else",
			"blankassertion", []string{
				"main.go:12:6: unused exported type: Marker",
				"main.go:14:6: unused exported type: Widget",
				"main.go:22:6: unused exported type: Hooks",
				"main.go:34:6: unused exported type: Node",
				"main.go:12:24: unused interface method: Marker.Mark",
				"main.go:16:15: unused exported method: Widget.Mark",
				"main.go:24:14: unused exported method: Hooks.Mark",
				"main.go:36:14: unused exported method: Node.Mark",
			}),

		// A //go:linkname push makes the body run under a name no source mentions, so
		// it is an entry point rather than dead code — including whatever it alone
		// reaches.
		Entry("roots a function a linkname directive publishes",
			"linkname", []string(nil)),

		// A //line directive renames a region to a file no caller opened. Findings
		// carry the name the loader read instead, which is the only one a caller can
		// look them up by.
		Entry("places a finding by the file the loader opened",
			"linedirective", []string{
				"main.go:8:6: unreachable func: Dead",
				"main.go:8:6: unused exported func: Dead",
			}),

		// With no main package and no declared API the whole exported surface stands
		// in as the roots, and is reported as unused alongside everything it cannot
		// reach. Internal packages contribute no roots of their own.
		Entry("stands the exported surface in as roots and reports it too",
			"library", []string{
				"internal/util/util.go:8:6: unreachable func: Dangling",
				"internal/util/util.go:8:6: unused exported func: Dangling",
				"lib.go:7:6: unused exported func: Greet",
				"lib.go:11:6: unused exported func: Unused",
			}),
	)

	It("roots an example with no output comment, which the test main never registers", func(ctx SpecContext) {
		Expect(analyzeWith(ctx, "tests", engine.Config{Tests: true})).To(BeEmpty())
	})

	It("leaves test-only references unseen when tests are excluded", func(ctx SpecContext) {
		Expect(analyzeWith(ctx, "tests", engine.Config{})).To(Equal([]string{
			"main.go:4:6: unreachable func: Compute",
			"main.go:4:6: unused exported func: Compute",
		}))
	})

	It("analyzes the files a build tag brings in", func(ctx SpecContext) {
		Expect(analyzeFixture(ctx, "buildtags")).To(BeEmpty())
		Expect(analyzeWith(ctx, "buildtags", engine.Config{BuildTags: []string{"integration"}})).To(Equal([]string{
			"tagged.go:6:6: unreachable func: Tagged",
			"tagged.go:6:6: unused exported func: Tagged",
		}))
	})

	It("defaults to ./... in the working directory", func(ctx SpecContext) {
		chdir(fixtureDir("control"))

		findings, err := engine.Analyze(ctx, engine.Config{})

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(HaveLen(1))
		Expect(findings[0].Name).To(Equal("Gauge.Read"))
	})

	Describe("the declared api surface", func() {
		It("reports everything when nothing is declared", func(ctx SpecContext) {
			Expect(analyzeFixture(ctx, "apisurface")).To(Equal([]string{
				"pkg/pkg.go:9:15: unreachable func: Widget.Spin",
				"pkg/pkg.go:11:6: unreachable func: spinHelper",
				"pkg/pkg.go:17:6: unreachable func: orphanUnexported",
				"pkg/pkg.go:6:6: unused exported type: Widget",
				"pkg/pkg.go:14:6: unused exported func: Emit",
				"pkg/pkg.go:9:15: unused exported method: Widget.Spin",
			}))
		})

		It("shields every exempt kind and leaves unexported code reported", func(ctx SpecContext) {
			// An exemption stands in for the consumers a run cannot see, and no consumer
			// can name an unexported declaration.
			Expect(analyzeWith(ctx, "apisurface", engine.Config{
				API:       []string{"./..."},
				APIExempt: []engine.Kind{engine.KindMethod, engine.KindFunc, engine.KindType},
			})).To(Equal([]string{
				"pkg/pkg.go:17:6: unreachable func: orphanUnexported",
			}))
		})

		It("keeps an exempt type's methods rooted, so its private callees stay live", func(ctx SpecContext) {
			Expect(analyzeWith(ctx, "apisurface", engine.Config{
				API:       []string{"./..."},
				APIExempt: []engine.Kind{engine.KindType},
			})).To(Equal([]string{
				"pkg/pkg.go:17:6: unreachable func: orphanUnexported",
				"pkg/pkg.go:14:6: unused exported func: Emit",
				"pkg/pkg.go:9:15: unused exported method: Widget.Spin",
			}))
		})

		It("rejects api exemptions with no api patterns", func(ctx SpecContext) {
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:       fixtureDir("control"),
				Patterns:  []string{"./..."},
				APIExempt: []engine.Kind{engine.KindMethod},
			})

			Expect(err).To(MatchError(ContainSubstring("api exemptions require api patterns")))
		})

		It("rejects an unknown api exemption", func(ctx SpecContext) {
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:       fixtureDir("control"),
				Patterns:  []string{"./..."},
				API:       []string{"./..."},
				APIExempt: []engine.Kind{"nonsense"},
			})

			Expect(err).To(MatchError(ContainSubstring(`unknown api exemption "nonsense"`)))
		})

		It("rejects an api pattern that resolves to nothing", func(ctx SpecContext) {
			// The pattern comes back from the loader as a package in its own right, so
			// without a look at its errors the surface would shield nothing and say
			// nothing.
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:      fixtureDir("control"),
				Patterns: []string{"./..."},
				API:      []string{"./nope"},
			})

			Expect(err).To(MatchError(ContainSubstring("resolving api pattern ./nope")))
		})

		It("rejects an api pattern that resolves to nothing among ones that do", func(ctx SpecContext) {
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:      fixtureDir("control"),
				Patterns: []string{"./..."},
				API:      []string{"./...", "./nope/..."},
			})

			Expect(err).To(MatchError(ContainSubstring("resolving api pattern ./nope/...")))
		})

		It("exempts a method on an api package's live type", func(ctx SpecContext) {
			findings, err := engine.Analyze(ctx, engine.Config{
				Dir:       fixtureDir("control"),
				Patterns:  []string{"./..."},
				API:       []string{"./..."},
				APIExempt: []engine.Kind{engine.KindMethod},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(BeEmpty())
		})
	})

	Describe("failing rather than degrading", func() {
		It("refuses a program with nothing to root at", func(ctx SpecContext) {
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:      fixtureDir("noroots"),
				Patterns: []string{"./internal/..."},
			})

			Expect(err).To(MatchError(engine.ErrNoRoots))
		})

		It("refuses a program that does not type check", func(ctx SpecContext) {
			// An unparsed caller looks like no caller, so a partial load would yield
			// findings that are wrong rather than merely incomplete.
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:      fixtureDir("broken"),
				Patterns: []string{"./..."},
			})

			Expect(err).To(MatchError(ContainSubstring("undefined: undefinedHelper")))
		})

		It("refuses to answer past a canceled context", func(ctx SpecContext) {
			canceled, cancel := context.WithCancel(ctx)
			cancel()

			_, err := engine.Analyze(canceled, engine.Config{
				Dir:      fixtureDir("control"),
				Patterns: []string{"./..."},
			})

			Expect(err).To(MatchError(ContainSubstring("context canceled")))
		})
	})

	It("resolves positions the caller can place in another FileSet", func(ctx SpecContext) {
		findings, err := engine.Analyze(ctx, engine.Config{
			Dir:      fixtureDir("control"),
			Patterns: []string{"./..."},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(HaveLen(1))
		Expect(findings[0].Kind).To(Equal(engine.KindMethod))
		Expect(findings[0].Verdict).To(Equal(engine.VerdictReflectionLiveMethod))
		Expect(findings[0].Package).To(Equal("control"))
		Expect(filepath.IsAbs(findings[0].Pos.Filename)).To(BeTrue())
		// A byte offset is what lets another FileSet place the finding.
		Expect(findings[0].Pos.Offset).To(BeNumerically(">", 0))
	})
})

func fixtureDir(name string) string {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())
	return dir
}

func chdir(dir string) {
	GinkgoHelper()

	previous, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chdir(dir)).To(Succeed())
	DeferCleanup(func() { Expect(os.Chdir(previous)).To(Succeed()) })
}

func analyzeFixture(ctx SpecContext, name string) []string {
	GinkgoHelper()

	return analyzeWith(ctx, name, engine.Config{})
}

// analyzeWith runs the engine over one fixture module and renders its findings
// the way a command would, so an expectation reads like the output a user would
// see.
func analyzeWith(ctx SpecContext, name string, cfg engine.Config) []string {
	GinkgoHelper()

	dir := fixtureDir(name)
	cfg.Dir = dir
	cfg.Patterns = []string{"./..."}
	findings, err := engine.Analyze(ctx, cfg)
	Expect(err).NotTo(HaveOccurred())

	var reported []string //nolint:prealloc // a fixture reporting nothing is asserted as []string(nil), which preallocation would turn into an empty non-nil slice.
	for _, finding := range findings {
		rel, err := filepath.Rel(dir, finding.Pos.Filename)
		Expect(err).NotTo(HaveOccurred())
		reported = append(reported, fmt.Sprintf("%s:%d:%d: %s: %s",
			rel, finding.Pos.Line, finding.Pos.Column, finding.Verdict, finding.Name))
	}
	return reported
}
