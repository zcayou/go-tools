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

		// Holder is the fixture's one conversion, so ViaMethod arrives in the closure
		// derived off Holder's exported Make, while ViaFunc — a free function's
		// result — never arrives at all. Derivation credits ViaMethod.Label,
		// the exported method reflection can reach there, and credits neither src:
		// an unexported method is weighed against the seeds. read and label
		// are unreachable, which is what makes the point that their selections still
		// count — dispatch credit is syntactic, so the two src verdicts turn
		// on the evidence gate alone.
		Entry("does not let derivation credit an unexported method it cannot reach",
			"derivedcredit", []string{
				"main.go:7:6: unreachable func: read",
				"main.go:13:6: unreachable func: label",
				"main.go:29:18: unreachable func: ViaMethod.src",
				"main.go:36:16: unreachable func: ViaFunc.src",
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
		// ExampleCompute and its helper draw no plain verdict — the example is rooted
		// even though nothing registers it — while Compute, which only the example
		// reaches, surfaces as test-only rather than passing as live.
		Expect(analyzeWith(ctx, "tests", engine.Config{Tests: true})).To(Equal([]string{
			"main.go:4:6: test-only unreachable func: Compute",
			"main.go:4:6: test-only unused exported func: Compute",
		}))
	})

	It("leaves test-only references unseen when tests are excluded", func(ctx SpecContext) {
		Expect(analyzeWith(ctx, "tests", engine.Config{})).To(Equal([]string{
			"main.go:4:6: unreachable func: Compute",
			"main.go:4:6: unused exported func: Compute",
		}))
	})

	Describe("test-only liveness", func() {
		// Under tests:true every verdict is evaluated twice over the same loaded
		// program — once with all evidence, once with test-origin evidence masked —
		// and a production declaration reported only under the mask is what test code
		// alone keeps alive.
		It("reports production declarations only test evidence keeps alive", func(ctx SpecContext) {
			Expect(analyzeWith(ctx, "testonly", engine.Config{Tests: true})).To(Equal([]string{
				// The full view first, byte-identical to what tests:true reported before
				// the family existed: dead test code is judged there alone, and Bark —
				// selected by nothing in either view — keeps its plain verdict.
				"main_test.go:19:6: unreachable func: testHelper",
				"main.go:14:12: unused reflection-live exported method: Dog.Bark",
				// Then the masked view's family: the interface method whose only selection
				// sits in the test file, the implementation that selection
				// dispatch-credited, and every ident only tests reference. TestSpeak itself
				// — unreachable under the mask — draws nothing.
				"main.go:11:12: test-only unreachable func: Dog.Speak",
				"main.go:4:6: test-only unused exported type: Speaker",
				"main.go:17:7: test-only unused exported const: Threshold",
				"main.go:19:5: test-only unused exported var: Registry",
				"main.go:21:6: test-only unused exported type: Mode",
				"main.go:5:2: test-only unused interface method: Speaker.Speak",
				"main.go:11:12: test-only unused exported method: Dog.Speak",
			}))
		})

		It("exempts the api surface in both views, before the diff", func(ctx SpecContext) {
			// The exempt idents draw nothing even though only tests reference them,
			// and the api roots keep Dog.Speak reachable in the masked view — its
			// unused-method verdict is what survives.
			Expect(analyzeWith(ctx, "testonly", engine.Config{
				Tests:     true,
				API:       []string{"./..."},
				APIExempt: []engine.Kind{engine.KindType, engine.KindConst, engine.KindVar},
			})).To(Equal([]string{
				"main_test.go:19:6: unreachable func: testHelper",
				"main.go:14:12: unused exported method: Dog.Bark",
				"main.go:5:2: test-only unused interface method: Speaker.Speak",
				"main.go:11:12: test-only unused exported method: Dog.Speak",
			}))
		})

		It("reports plain verdicts and no family at all when tests are excluded", func(ctx SpecContext) {
			Expect(analyzeWith(ctx, "testonly", engine.Config{})).To(Equal([]string{
				"main.go:11:12: unreachable func: Dog.Speak",
				"main.go:14:12: unreachable func: Dog.Bark",
				"main.go:4:6: unused exported type: Speaker",
				"main.go:17:7: unused exported const: Threshold",
				"main.go:19:5: unused exported var: Registry",
				"main.go:21:6: unused exported type: Mode",
				"main.go:5:2: unused interface method: Speaker.Speak",
				"main.go:11:12: unused exported method: Dog.Speak",
				"main.go:14:12: unused exported method: Dog.Bark",
			}))
		})

		It("roots nothing for a credit whose bind only tests reach", func(ctx SpecContext) {
			// describe binds Label to fmt.Stringer in production code, and only the test
			// calls describe. The bind spares String's own verdict in the masked view,
			// and no more: rooting String would carry render out of the family
			// on the strength of a conversion nothing but a test runs.
			Expect(analyzeWith(ctx, "testbind", engine.Config{Tests: true})).To(Equal([]string{
				"main.go:13:6: test-only unreachable func: render",
				"main.go:15:6: test-only unreachable func: describe",
			}))
		})
	})

	Describe("a contract a test file declares", func() {
		// Whose an interface is turns on whether the run loads it, not on which file
		// it sits in. A _test.go contract's call sites are all in the loaded program,
		// so the conditional rule applies to it exactly as it does to a production
		// interface, and the dead pair is reported together rather than crediting
		// itself away.
		It("weighs it like any interface the analyzed packages own", func(ctx SpecContext) {
			Expect(analyzeWith(ctx, "testcontract", engine.Config{Tests: true})).To(Equal([]string{
				// register binds fixture to contract, and nothing selects sample, so
				// the bind confers nothing and both halves are reported. The used pair
				// beside it carries the same shape with take selected, which is what
				// keeps this from asserting that a test contract never confers.
				"main_test.go:16:16: unreachable func: fixture.sample",
				"main_test.go:11:2: unused interface method: contract.sample",
			}))
		})

		It("reports nothing once the test files are out of the program", func(ctx SpecContext) {
			Expect(analyzeWith(ctx, "testcontract", engine.Config{})).To(BeNil())
		})
	})

	Describe("declared test-facing packages", func() {
		// The kit package is production-shaped and consumed only by the main
		// package's test file, through a production declaration of its own
		// that nothing else reaches.
		It("pins what the declaration buys: the family names the package and dims the trail", func(ctx SpecContext) {
			// Without the declaration kit's whole surface draws the family — being alive
			// only through tests is its job — and lib.Only draws the missing-root
			// signature, unreachable without the unused half, because kit's reference
			// still counts.
			Expect(analyzeWith(ctx, "testfacing", engine.Config{Tests: true})).To(Equal([]string{
				"kit/kit.go:11:6: unreachable func: orphan",
				"kit/kit.go:8:6: test-only unreachable func: Greet",
				"lib/lib.go:8:6: test-only unreachable func: Only",
				"kit/kit.go:8:6: test-only unused exported func: Greet",
			}))
		})

		It("extends test origin by declaration and sharpens what lies downstream", func(ctx SpecContext) {
			// kit's declarations are judged in the full view alone: Greet draws nothing,
			// while orphan — dead even with tests counted — keeps its plain verdict.
			// lib.Only loses kit's reference along with its reachability, so it draws
			// both halves of the family.
			Expect(analyzeWith(ctx, "testfacing", engine.Config{
				Tests:      true,
				TestFacing: []string{"./kit"},
			})).To(Equal([]string{
				"kit/kit.go:11:6: unreachable func: orphan",
				"lib/lib.go:8:6: test-only unreachable func: Only",
				"lib/lib.go:8:6: test-only unused exported func: Only",
			}))
		})

		It("rejects test-facing without tests", func(ctx SpecContext) {
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:        fixtureDir("testfacing"),
				Patterns:   []string{"./..."},
				TestFacing: []string{"./kit"},
			})

			Expect(err).To(MatchError(ContainSubstring("test-facing requires tests")))
		})

		It("rejects a test-facing pattern that resolves to nothing", func(ctx SpecContext) {
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:        fixtureDir("testfacing"),
				Patterns:   []string{"./..."},
				Tests:      true,
				TestFacing: []string{"./nope"},
			})

			Expect(err).To(MatchError(ContainSubstring("resolving test-facing pattern ./nope")))
		})

		It("rejects a package declared both api and test-facing", func(ctx SpecContext) {
			// The two assert contradictory facts about who the package's consumers are.
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:        fixtureDir("testfacing"),
				Patterns:   []string{"./..."},
				Tests:      true,
				API:        []string{"./kit"},
				TestFacing: []string{"./kit"},
			})

			Expect(err).To(MatchError(ContainSubstring("declared both api and test-facing: testfacing/kit")))
		})
	})

	Describe("declared root programs", func() {
		// The tools directory holds two colliding ignore-tagged generators —
		// the convention's normal one-directory form, unreachable through build-tags
		// — and lib is reached almost entirely by them.
		It("pins the blind spot: an invisible entry point silences nothing", func(ctx SpecContext) {
			Expect(analyzeFixture(ctx, "declaredroots")).To(Equal([]string{
				"lib/lib.go:5:6: unreachable func: Generate",
				"lib/lib.go:8:6: unreachable func: Sweep",
				"lib/lib.go:11:6: unreachable func: Untouched",
				"lib/lib.go:13:6: unreachable func: helper",
				"lib/lib.go:5:6: unused exported func: Generate",
				"lib/lib.go:8:6: unused exported func: Sweep",
				"lib/lib.go:11:6: unused exported func: Untouched",
			}))
		})

		It("loads each declared file as its own program: roots and evidence, never findings", func(ctx SpecContext) {
			// Both generators load side by side despite colliding declarations, their
			// call edges and references count, the declaration no generator reaches
			// keeps its findings, and nothing in a root program is ever reported.
			Expect(analyzeWith(ctx, "declaredroots", engine.Config{
				Roots: []string{"tools/*.go"},
			})).To(Equal([]string{
				"lib/lib.go:11:6: unreachable func: Untouched",
				"lib/lib.go:11:6: unused exported func: Untouched",
			}))
		})

		It("roots the generators in the masked view too", func(ctx SpecContext) {
			// Generate is reached by a test and a generator both. A root program
			// is a production consumer, so under the mask Generate stays reachable
			// and referenced — it draws nothing in either view.
			Expect(analyzeWith(ctx, "declaredroots", engine.Config{
				Tests: true,
				Roots: []string{"tools/*.go"},
			})).To(Equal([]string{
				"lib/lib.go:11:6: unreachable func: Untouched",
				"lib/lib.go:11:6: unused exported func: Untouched",
			}))
		})

		It("rejects a roots path naming a missing file", func(ctx SpecContext) {
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:      fixtureDir("declaredroots"),
				Patterns: []string{"./..."},
				Roots:    []string{"tools/nope.go"},
			})

			Expect(err).To(MatchError(ContainSubstring(`roots pattern "tools/nope.go" matched no files`)))
		})

		It("rejects a roots glob matching nothing", func(ctx SpecContext) {
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:      fixtureDir("declaredroots"),
				Patterns: []string{"./..."},
				Roots:    []string{"gen/*.go"},
			})

			Expect(err).To(MatchError(ContainSubstring(`roots pattern "gen/*.go" matched no files`)))
		})

		It("rejects a declared file that is not a main program", func(ctx SpecContext) {
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:      fixtureDir("declaredroots"),
				Patterns: []string{"./..."},
				Roots:    []string{"lib/lib.go"},
			})

			Expect(err).To(MatchError(ContainSubstring("root program lib/lib.go declares package lib, not main")))
		})

		It("rejects a root program with no func main", func(ctx SpecContext) {
			// go/types does not require one — that is the linker's rule — and without
			// this check the file would contribute evidence but no root, a silent
			// half-effect.
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:      fixtureDir("declaredroots"),
				Patterns: []string{"./..."},
				Roots:    []string{"partial/prog.go"},
			})

			Expect(err).To(MatchError(ContainSubstring("root program partial/prog.go declares no func main")))
		})

		It("refuses to run while the reserved directory exists on disk", func(ctx SpecContext) {
			reserved := filepath.Join(fixtureDir("declaredroots"), ".deadcode-roots")
			Expect(os.Mkdir(reserved, 0o750)).To(Succeed())
			DeferCleanup(func() { Expect(os.Remove(reserved)).To(Succeed()) })

			_, err := engine.Analyze(ctx, engine.Config{
				Dir:      fixtureDir("declaredroots"),
				Patterns: []string{"./..."},
				Roots:    []string{"tools/*.go"},
			})

			Expect(err).To(MatchError(ContainSubstring("reserved for synthesized root programs")))
		})
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

		Describe("a generic surface", func() {
			// The fixture is a library whose only way in is generic, so the exported
			// non-generic Describe is all the masked view can root without help.
			generic := func(rooting engine.GenericRooting) engine.Config {
				return engine.Config{
					Tests:       true,
					API:         []string{"./..."},
					APIExempt:   []engine.Kind{engine.KindMethod, engine.KindFunc, engine.KindType},
					APIGenerics: rooting,
				}
			}

			It("leaves what a generic API alone reaches unrooted by default", func(ctx SpecContext) {
				Expect(analyzeWith(ctx, "apigenerics", generic(engine.GenericRootingSkip))).To(Equal([]string{
					"lib/lib.go:55:6: unreachable func: rareHelper",
					"lib/lib.go:63:6: unreachable func: orphan",
					"lib/lib.go:93:6: unreachable func: pendingHelper",
					"lib/lib.go:33:6: test-only unreachable func: retain",
					"lib/lib.go:35:6: test-only unreachable func: prepare",
					"lib/lib.go:37:6: test-only unreachable func: observe",
					"lib/lib.go:45:6: test-only unreachable func: auditHelper",
					"lib/lib.go:74:19: test-only unreachable func: Value.source",
					"lib/lib.go:84:6: test-only unreachable func: fakeHelper",
				}))
			})

			It("roots the instantiations the program builds of it", func(ctx SpecContext) {
				// Only the test instantiates, and it is masked, so this is also the claim
				// that instantiations are gathered from the whole program.
				Expect(analyzeWith(ctx, "apigenerics", generic(engine.GenericRootingInstantiated))).To(Equal([]string{
					"lib/lib.go:63:6: unreachable func: orphan",
					"lib/lib.go:60:6: unmeasured generic: Opaque",
					"lib/lib.go:45:6: test-only unreachable func: auditHelper",
					"lib/lib.go:84:6: test-only unreachable func: fakeHelper",
				}))
			})

			It("measures the generic surface when the rooting is left unset", func(ctx SpecContext) {
				// A declared surface that is generic is what the setting exists for, so
				// an absent one asks to measure it rather than to report it as dead.
				Expect(analyzeWith(ctx, "apigenerics", engine.Config{
					Tests:     true,
					API:       []string{"./..."},
					APIExempt: []engine.Kind{engine.KindMethod, engine.KindFunc, engine.KindType},
				})).To(Equal(analyzeWith(ctx, "apigenerics", generic(engine.GenericRootingInstantiated))))
			})

			It("roots a generic nothing instantiates through the calls its body makes", func(ctx SpecContext) {
				// Rare is never instantiated, so no monomorphized body exists. Its origin
				// body calls rareHelper, and it calls it whatever a consumer would
				// instantiate it with, so rareHelper is reachable rather than dead.
				Expect(analyzeWith(ctx, "apigenerics", generic(engine.GenericRootingInstantiated))).
					NotTo(ContainElement(ContainSubstring("rareHelper")))
			})

			It("names the generic it could not follow rather than reporting past it", func(ctx SpecContext) {
				// Opaque calls through a function value, which needs the type flow only
				// an instantiation carries. Saying so is a different claim from calling
				// what lies beyond it dead, and no exemption covers it: the point
				// of the verdict is that an exempt declaration would otherwise go
				// unmentioned.
				Expect(analyzeWith(ctx, "apigenerics", generic(engine.GenericRootingInstantiated))).
					To(ContainElement("lib/lib.go:60:6: unmeasured generic: Opaque"))
			})

			It("walks an exported method of a generic type nothing instantiates", func(ctx SpecContext) {
				// x/tools builds no method value for a parameterized receiver, so Pending's
				// method set yields nothing to walk; Count is found through its
				// declaration.
				Expect(analyzeWith(ctx, "apigenerics", generic(engine.GenericRootingInstantiated))).
					NotTo(ContainElement(ContainSubstring("pendingHelper")))
			})

			It("weighs a sealed contract against the test's instantiation in the masked view too", func(ctx SpecContext) {
				// Source is concrete only where the test's call makes it so. Which type
				// argument a consumer picks says nothing about who holds the interface —
				// the reading instantiated rooting takes of the same test — so Value.source
				// is credited in both views rather than drawing the test-only family.
				Expect(analyzeWith(ctx, "apigenerics", generic(engine.GenericRootingInstantiated))).
					NotTo(ContainElement(ContainSubstring("Value.source")))
			})

			It("lets no test's instantiation shape a sealed contract under skip", func(ctx SpecContext) {
				// skip declines the reading altogether: in the masked view Source
				// is instantiated nowhere, so Value.source is what only the test keeps
				// alive.
				Expect(analyzeWith(ctx, "apigenerics", generic(engine.GenericRootingSkip))).
					To(ContainElement("lib/lib.go:74:19: test-only unreachable func: Value.source"))
			})

			It("roots no credited method whose body the masked view counts as test code", func(ctx SpecContext) {
				// The seal credits the test's fake in both views, and a credited method
				// is a root. Rooted in the masked view, the fake would carry fakeHelper
				// back out of the test-only family.
				Expect(analyzeWith(ctx, "apigenerics", generic(engine.GenericRootingInstantiated))).
					To(ContainElement("lib/lib.go:84:6: test-only unreachable func: fakeHelper"))
			})

			It("still draws the test-only family on what no instantiation reaches", func(ctx SpecContext) {
				// The instantiation a test writes stands in for a consumer's type argument
				// and for nothing else. auditHelper is production code the test calls
				// directly, no path from the public surface arrives at it, and it stays
				// reported — supplying a type argument is not a use.
				Expect(analyzeWith(ctx, "apigenerics", generic(engine.GenericRootingInstantiated))).
					To(ContainElement("lib/lib.go:45:6: test-only unreachable func: auditHelper"))
			})

			It("rejects a generic rooting with no api patterns", func(ctx SpecContext) {
				_, err := engine.Analyze(ctx, engine.Config{
					Dir:         fixtureDir("control"),
					Patterns:    []string{"./..."},
					APIGenerics: engine.GenericRootingInstantiated,
				})

				Expect(err).To(MatchError(ContainSubstring("api generic rooting requires api patterns")))
			})

			It("rejects an unknown generic rooting", func(ctx SpecContext) {
				_, err := engine.Analyze(ctx, engine.Config{
					Dir:         fixtureDir("control"),
					Patterns:    []string{"./..."},
					API:         []string{"./..."},
					APIGenerics: "nonsense",
				})

				Expect(err).To(MatchError(ContainSubstring(`unknown api generic rooting "nonsense"`)))
			})
		})

		Describe("a sealed surface", func() {
			// lib seals its interfaces with an unexported method, so no package but lib
			// can implement them and the conversion that puts a handle behind one
			// belongs to a consumer. hidden holds the same shape outside the surface.
			sealed := func(patterns ...string) engine.Config {
				return engine.Config{
					API:       patterns,
					APIExempt: []engine.Kind{engine.KindMethod, engine.KindFunc, engine.KindType},
				}
			}

			It("reports every implementation while no surface is declared", func(ctx SpecContext) {
				// Sealing alone credits nothing. The fallback root set measures a library
				// rather than shielding one, so standing in for consumers takes saying
				// they exist.
				Expect(analyzeFixture(ctx, "sealedsurface")).To(ContainElements(
					"lib/lib.go:14:14: unreachable func: Ref.input",
					"lib/lib.go:22:20: unreachable func: Tagged.input",
					"lib/lib.go:33:18: unreachable func: Binding.bound",
					"lib/lib.go:48:20: unreachable func: StringKey.key",
				))
			})

			It("credits what only a consumer could convert once the surface is declared", func(ctx SpecContext) {
				// No exemption reaches these: the exempt kinds cover the exported surface,
				// and a sealing method is unexported by construction. Tagged.input proves
				// the credit reaches a generic receiver, and Binding.Name that a sealed
				// interface credits the exported methods a consumer holding it can call.
				// What the credited methods call is live with them, and Lazy.input
				// is the one whose calls the analysis could not follow.
				Expect(analyzeWith(ctx, "sealedsurface", sealed("./..."))).To(Equal([]string{
					"lib/lib.go:40:20: unreachable func: halfBound.bound",
					"lib/lib.go:56:19: unreachable func: IntTaken.taken",
					"lib/lib.go:52:39: unused interface method: Untaken.taken",
					"lib/lib.go:93:18: unmeasured generic: Lazy.input",
				}))
			})

			It("reaches what a credited method calls", func(ctx SpecContext) {
				// Ref.input runs whenever a consumer's Consume does, and RTA sees neither
				// the conversion nor the dispatch. Its credit spares the method's own
				// verdict; rooting it is what spares canonical.
				Expect(analyzeWith(ctx, "sealedsurface", sealed("./..."))).
					NotTo(ContainElement(ContainSubstring("canonical")))
			})

			It("walks a credited method nothing instantiates for what it calls", func(ctx SpecContext) {
				// No Tagged is ever built here, so its credited input has no concrete body
				// to root. Its generic body calls tagID whatever a consumer instantiates
				// it with, the way an uninstantiated generic on the surface is walked.
				Expect(analyzeWith(ctx, "sealedsurface", sealed("./..."))).
					NotTo(ContainElement(ContainSubstring("tagID")))
			})

			It("names a credited method it could not follow, unexported as it is", func(ctx SpecContext) {
				// Lazy.input calls through a function value, which a walk cannot follow
				// without an instantiation's type flow. The verdict that says so names
				// an unexported method as readily as the surface's own generics.
				Expect(analyzeWith(ctx, "sealedsurface", sealed("./..."))).
					To(ContainElement("lib/lib.go:93:18: unmeasured generic: Lazy.input"))
			})

			It("holds each implementation behind the seal for the other credits to find", func(ctx SpecContext) {
				// Install asserts what it was handed to carrier. The consumer's conversion
				// to Supplier is what puts a Group behind an interface, so the assertion
				// finds Group.carried the way it finds a converted operand.
				Expect(analyzeWith(ctx, "sealedsurface", sealed("./..."))).
					NotTo(ContainElement(ContainSubstring("Group.carried")))
			})

			It("credits by satisfaction rather than by method name", func(ctx SpecContext) {
				// halfBound carries the seal and not Bound's exported half, so
				// it implements the contract nowhere and the contract credits it nowhere.
				Expect(analyzeWith(ctx, "sealedsurface", sealed("./..."))).
					To(ContainElement("lib/lib.go:40:20: unreachable func: halfBound.bound"))
			})

			It("weighs a generic contract per instantiation and reports what has none", func(ctx SpecContext) {
				// Keyed[string] is a shape the program builds, so StringKey.key is credited
				// against it. Untaken is instantiated nowhere, so there is no concrete
				// interface to weigh IntTaken.taken against — and the pair says so, naming
				// the contract alongside the implementation.
				findings := analyzeWith(ctx, "sealedsurface", sealed("./..."))
				Expect(findings).NotTo(ContainElement(ContainSubstring("StringKey.key")))
				Expect(findings).To(ContainElements(
					"lib/lib.go:56:19: unreachable func: IntTaken.taken",
					"lib/lib.go:52:39: unused interface method: Untaken.taken",
				))
			})

			It("credits nothing in a package the surface does not name", func(ctx SpecContext) {
				// hidden seals Gate exactly as lib seals Input. Declaring one package
				// says nothing about who consumes another.
				Expect(analyzeWith(ctx, "sealedsurface", sealed("./lib"))).To(ContainElements(
					"hidden/hidden.go:9:16: unreachable func: Entry.gate",
					"hidden/hidden.go:7:6: unused exported type: Entry",
				))
			})

			It("leaves an unsealed interface weighed as it always was", func(ctx SpecContext) {
				// Open declares an exported method set, which any package can satisfy, so
				// nothing about the surface closes its implementations. Plain.Label
				// is silent here because the exempt kinds cover an exported method, which
				// is the answer that already existed.
				Expect(analyzeWith(ctx, "sealedsurface", engine.Config{API: []string{"./..."}})).
					To(ContainElement("lib/lib.go:64:14: unused exported method: Plain.Label"))
			})
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

		It("refuses a program whose only roots are tests", func(ctx SpecContext) {
			// The full view roots at the test entries, but the masked question — what
			// does the program need with no test vouching for it — has nothing to root
			// at, and reachability is undefined rather than empty. An empty test-only
			// family would read as all clear, which is the one answer this run cannot
			// support.
			_, err := engine.Analyze(ctx, engine.Config{
				Dir:      fixtureDir("testrooted"),
				Patterns: []string{"./..."},
				Tests:    true,
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
