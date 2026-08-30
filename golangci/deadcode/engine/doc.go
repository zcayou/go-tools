// Package engine finds code that nothing can reach.
//
// It is the analysis itself, with no opinion about how findings are presented.
// The parent package adapts the whole-program result to go/analysis's
// per-package contract and is its only caller. They are separate packages
// because each names the same idea differently: [Config] is what the analysis
// needs, and the parent's Settings is a decoded YAML block.
//
// The engine performs Rapid Type Analysis over the loaded program to find
// unreachable functions, and augments it with four reference scans: exported
// package-level functions, types, constants and variables that nothing
// references, interface methods nothing selects, exported methods reachability
// keeps live only through reflection, and exported methods nothing selects
// or binds. One declaration can draw more than one verdict; [Verdict]
// enumerates them.
//
// # Why this is not a go/analysis analyzer
//
// The engine loads and analyzes a whole program at once, which go/analysis
// is deliberately not built for. Three properties force it:
//
//   - Reachability flows from roots to callees, which is from a package to its
//     dependencies. Analysis facts flow the other way, from dependencies
//     to dependents, so the result cannot be carried by facts.
//   - A verdict about a declaration depends on every package that could
//     reference it, including packages that import it. A per-package pass
//     cannot see them.
//   - golangci-lint drops the synthesized <pkg>.test main packages before
//     analysis, so a driver that only sees its package set cannot root at test
//     binaries.
//
// The engine therefore does its own package loading, and each frontend adapts
// the result. See the plugin package for what that costs a golangci-lint run.
//
// # Roots
//
// Roots decide what alive means, and they accumulate from whatever entry points
// exist rather than being selected by a mode. Main packages are roots, which
// including tests extends with the synthesized test binaries and with the test
// functions cmd/go runs — an example carrying no output comment is compiled
// and never registered, so nothing reaches it through the generated main.
// Declared API packages contribute their exported declarations as roots, so
// what an exempt declaration alone reaches stays live. Only when both come up
// empty does the exported surface of every loaded package stand in, so
// that a library analyzes at all.
//
// That fallback is a way to measure a library, not a way to shield one.
// Standing in as roots keeps a package's public surface from being reported
// as unreachable, and does nothing about the reference scans: a library
// configured with no API reports its whole public surface as unused, because
// within the loaded program nothing references it. Declaring the API is what
// distinguishes a surface consumers reach from one nothing does.
//
// A function a //go:linkname directive publishes under another package's symbol
// is a root wherever it appears, and is not reported: the body runs under
// a name no reference in source mentions.
//
// A file [Config.Roots] declares is loaded as its own single-file main program
// — the go run model, for the conventional package main file behind //go:build
// ignore. The default load structurally cannot see such a file, and build tags
// cannot admit it, because the tag is what lets several such programs share one
// directory; applied, the siblings collide and the directory stops type
// checking. The synthesized program's main roots in every view — a declared
// root is a production entry point, so it composes with either value
// of [Config.Tests] — and each fact in it counts as evidence through
// the ordinary scans, while its own declarations are never candidates and its
// packages are not among the analyzed set: an interface it declares credits
// unconditionally, the way a dependency's does. A declared entry point adds
// edges, and nothing else.
//
// Generic declarations cannot be rooted, because an uninstantiated body has no
// concrete instance and RTA rejects a type parameter as a runtime type.
// A generic-heavy API standing in for its own roots therefore leaves the code
// only it reaches reported as unreachable.
//
// [GenericRootingInstantiated] is the answer to that. It roots the concrete
// instantiations the program builds of the declared surface instead, which
// are ordinary concrete functions RTA reads like any other. The type arguments
// are not read as evidence: an instantiation calls the same declarations
// whichever a consumer picks, and the reachable set is taken from the body. So
// instantiations count wherever they were written, test files included —
// for a library, its own tests are usually the only code instantiating its
// public generics, and refusing them leaves the surface unrooted in exactly
// the masked view the question is asked in. What the argument does decide
// is which of its own methods become runtime types, so a method a test type
// alone selects is credited where the default would have reported it.
//
// A generic the program never instantiates has no monomorphized body to root
// at all. Its origin body still exists, and the calls it makes to concrete
// functions are the same calls whatever a consumer instantiates it with, so
// those callees are rooted directly and reachability carries on from them.
// Where that walk meets a call it cannot resolve — through an interface,
// or a function value — the generic draws [VerdictUnmeasuredGeneric] instead.
// That verdict reports the analysis rather than the declaration: it says
// an edge went unfollowed, which is why nothing past it is reported
// as unreachable on the strength of it. No exemption covers it, because
// an exempt declaration is exactly the one nothing else would mention.
//
// # What counts as a reference
//
// An exported identifier is reported as unused when its only references are its
// own declaration — methods on the type, recursive mentions — or an inert blank
// compile-time assertion, since neither is a use by anything else. Inert means
// both halves of var _ T = v: a declared type to check against, and a value
// the compiler can satisfy without running anything. A blank var whose value
// is computed runs that computation, and a reference inside a function literal
// counts wherever it appears, because that body runs: Ginkgo's var _ =
// Describe("...", func() { ... }) uses everything it names.
//
// # Interface participation
//
// A method counts as used when the program binds its receiver to an interface
// that declares it. Once the caller is a library, participation is all a static
// analysis can observe — it never sees log/slog invoke the handler it was
// given, a decoder invoke UnmarshalYAML on the value it was handed,
// or errors.Is walk an Unwrap chain — so a bind credits every method
// the interface declares.
//
// Four kinds of evidence establish a bind, because no single one covers
// the cases:
//
//   - A conversion to the interface, read from the SSA program. Generics
//     are already instantiated there, so a concrete type reaching a generic
//     interface is seen as the instantiated interface rather than
//     the parameterized one no concrete type implements.
//   - A type assertion or type switch case naming the interface, read from
//     the syntax of every loaded package, dependencies included. A library
//     discovers an optional capability in its own source, and SSA bodies
//     are built only for the analyzed packages. reflect.TypeAssert counts
//     as one: encoding/json and encoding/xml find Marshaler and TextMarshaler
//     through nothing else.
//   - A type argument satisfying an interface constraint at a generic
//     instantiation. A constraint is satisfied rather than converted, so no
//     conversion records it, but the compiler verified the argument against
//     it — the same evidence a conversion carries.
//   - A sealed interface the declared API surface exposes. An unexported
//     method is scoped to its own package, so no other package can supply one
//     and the implementations are closed and entirely in view; a consumer
//     reaching the declared surface has to convert one of them to hold
//     the interface at all, and that conversion sits in the consumer.
//
// Sealing is the one shape no exemption reaches. The exempt kinds stand
// in for consumers over the exported surface, and a sealing method
// is unexported by construction, so without the surface rule the standard idiom
// for a closed implementation set reports every implementation
// of it as unreachable — and returning the interface from the factory,
// the change that would make the conversion visible, is exactly what a typed
// handle cannot do. The credit needs the surface declared: sealing alone says
// nothing, because the fallback root set is a way to measure a library rather
// than to shield one. A generic sealed interface contributes the instantiations
// the program builds of it, since a parameterized interface is a shape no
// concrete method set matches, and one the program never instantiates
// contributes nothing.
//
// Structural satisfaction alone credits nothing: a dependency's interface
// that merely declares a method of the same name is not a use. Because
// an assertion names no concrete type, its credit is weighed against the types
// the program actually materializes behind an interface — without that, fmt
// asserting Stringer and error in its own source would spare every method
// of those names in every program that imports fmt, which is every program.
//
// Materialization is judged against the same evidence liveness uses:
// the derived-type closure of every conversion operand, computed
// with the derivation rules of RTA's addRuntimeType at the pinned x/tools
// version — struct fields, pointer, slice, array, chan and map constituents,
// map keys included, and the parameter and result types of signatures
// and of exported methods, with a named type's underlying and a signature's
// tuples traversed but granted nothing, because reflection cannot obtain
// a method set there. The closure additionally seeds from the resolved type
// arguments of reflect.TypeFor, whose descriptor no conversion ever carried.
// One evidence base for both sides is load-bearing: RTA keeps every exported
// method of every derived type alive, so a credit gate consulting anything
// narrower refuses exactly the methods liveness cannot explain — and alive but
// uncreditable is the false-positive space. That argument is about exported
// methods, because reflection reaches nothing else off a derived type, so
// an unexported method is credited against the closure's seeds alone —
// the conversion operands and reflect.TypeFor arguments — rather than
// everything derived from them. Otherwise a sealed interface's marker would
// be credited wherever its receiver happened to be the result type of some
// other type's exported method, which says nothing about whether the marker
// is ever invoked. A method of a generic type is judged by its origin, which
// the closure records alongside every instantiation that enters it.
//
// Type parameters in the evidence are resolved, never pattern-matched. Every
// generic object's fully concrete type-argument vectors are read from
// the loaded program's instantiations, dependencies included, and propagated
// transitively where an instantiation's own arguments are type parameters
// of the generic enclosing it. An interface asserted inside a generic body
// is specialized under each of those vectors before it is weighed,
// and a constraint method's signature is substituted the same way before
// it is matched, so evidence written against a type parameter credits exactly
// what the concrete instantiations demand. An unresolvable vector or a failed
// substitution credits nothing rather than guessing.
//
// Participation is one answer read by every method verdict: a method covered
// by bind or dispatch evidence is reported neither as an unused method nor
// as an unreachable function. The unreachable arm is a deliberate divergence
// from x/tools cmd/deadcode. A credited method is invoked through an interface
// RTA cannot see behind — the invoking call sits in a dependency body SSA never
// built, or behind a descriptor reflect.TypeFor conjured — so reporting
// it unreachable would assert what the participation verdicts just refused to.
//
// Interface-method liveness propagates across interface-to-interface flows.
// When interface A flows into interface B — a conversion, which identical
// method sets perform as ChangeType, exactly what an identically re-declared
// adapter produces — or A is supplied as a type argument satisfying
// an interface constraint B, a use of B's method is a use of A's declaration
// of that method: the declaration is required for the flow to compile,
// and dynamic dispatch through B enters whatever implementation sits behind A.
// Selections are the roots and propagation runs to a fixpoint, so a chain
// of adapters carries use all the way back. A type assertion between interfaces
// is deliberately not an edge: an assertion requires no static satisfaction, so
// the source interface's declaration is not load-bearing there.
//
// Whether a bind confers a use depends on whose interface it is. An interface
// a dependency declares confers unconditionally, because its call sites
// are beyond what this analysis reports on. An interface the analyzed packages
// declare confers only while that interface method is itself used — selected
// directly, or reached through a flow — which stops a dead interface method
// from laundering its implementations: the method is reported alongside them
// instead, and an adapter-shadowed declaration no longer strips credit from
// what implements it. An anonymous interface has no declaration to weigh
// and confers unconditionally, and so does a sealed one the declared surface
// exposes, for the reason a dependency's does: what a consumer selects through
// it is not in the loaded program. A sealed interface nothing exposes is still
// reported, as the unused exported type it is — the verdict that names
// the contract rather than the implementations satisfying it.
//
// Whose an interface is turns on whether this analysis loads it, not on which
// file it sits in. A contract declared in a _test.go file is the analyzed
// packages': its call sites are all in the loaded program, so a bind through
// it confers only while the contract is used, exactly as a production one does.
// Reporting is filtered separately — generated files draw no verdict, while
// a test file draws every verdict its declarations earn, which is what pairs
// a dead contract with the implementations satisfying it.
//
// # The analyzed set is an input, not just a filter
//
// Because participation is weighed against whether the interface's own liveness
// is measurable, the set of packages a run loads changes verdicts rather than
// only changing which findings are reported. Widening the load turns
// an interface from a dependency's into one of the analyzed packages', which
// downgrades its unconditional credit to a conditional one and can surface
// method findings a narrower run credited away. Two runs over different scopes
// are answering different questions, and neither is a subset of the other.
//
// # Test-only liveness
//
// Including tests makes test evidence circular for production code: a test
// exists because of the declaration it exercises, so it can never show
// the program needs it. Dead production code is usually dead-but-tested — its
// test was written when it had callers and outlived them — and under a single
// evaluation such a declaration is self-certifying, invisible forever.
//
// Under [Config.Tests] the engine therefore evaluates every verdict twice over
// the same loaded program: once with all evidence, and once with test-origin
// evidence masked. Test-origin means any fact positioned in a _test.go file —
// references, selections, conversions, assertions, instantiations,
// materializations, flow edges — together with the roots that exist only
// because of tests: the synthesized test mains and the test entry points,
// excluded by package identity because the in-package test variant shares
// the plain package's import path. A production declaration then lands
// in exactly one bucket: live in both views, and silent; reported in the full
// view, and reported exactly as a single evaluation would; or reported only
// under the mask, which surfaces its masked verdicts in their test-only form —
// test code is the only thing keeping it alive. Declarations in test
// and generated files are judged in the full view alone, and the API surface
// exempts in both views before the diff.
//
// Some test code cannot be spelled as test code: a test plugin looks, feels,
// and compiles exactly like a production plugin, and only who consumes it makes
// it test code. [Config.TestFacing] declares such packages into the definition
// of test origin. Under the mask a declared package's facts are removed exactly
// as _test.go facts are and it contributes no roots; its own declarations
// are judged in the full view alone — being alive only through tests is its
// job, while dead code inside it keeps its plain verdicts — and a production
// declaration whose only consumer it was loses the references along
// with the reachability, so it draws the complete family rather than
// the unreachable half a bare root exclusion could produce. Declaring the same
// package api is refused: the two assert contradictory facts about who its
// consumers are.
//
// The masked view is deliberately not a run without tests. As above, the loaded
// set is an input: a narrower load changes interface ownership and answers
// a different question. Masking holds the program constant, so the two
// evaluations differ in exactly one variable — whether test evidence exists.
//
// A program whose only entry points are tests is refused with [ErrNoRoots]:
// with test roots set aside there is nothing to root at, and reachability
// is undefined rather than empty. Declaring an API surface is what gives such
// a module production roots to measure against.
//
// # Load requirements
//
// The load must request syntax and type information for the whole transitive
// dependency graph, not only for the analyzed packages, because an assertion
// in a dependency's own source is evidence. In go/packages terms
// that is NeedDeps together with NeedSyntax and NeedTypesInfo, which loads
// every dependency from source and type checks its function bodies rather than
// reading export data.
//
// This is not negotiable: dropping it silently loses the library-caller
// evidence and returns findings that are wrong rather than merely incomplete.
// The cost is a load that scales with the whole dependency graph rather than
// with the packages under analysis, and it is the smaller half of the run —
// the SSA build and the reachability analysis that follow cost more.
package engine
