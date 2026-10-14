// Package testlayout checks that test files follow the repository's test-suite
// layout conventions.
//
// A test file is one of three kinds, and which one is not a matter of opinion.
// Beside a source, the package the file declares decides: a white-box file
// declares the package under test and can reach its unexported identifiers,
// and a black-box file declares the external <pkg>_test package and sees only
// what a consumer sees. In a directory holding no source to test there
// is nothing to be inside or outside of, and the file is standalone. Settings
// say which kinds a repository allows and what a file of each kind may
// be named, and the rules follow from that.
//
// # Rules
//
// Each diagnostic names the rule that produced it as the first word of its
// message, so a consumer can exclude one rule without silencing the linter:
//
//   - test-package: a test file of a kind the settings do not allow.
//   - test-file-name: a test file whose name no pattern for its kind allows.
//   - spec-less-test-file: a file named for the source it exercises carrying no
//     specs, which means the specs for that source are somewhere else.
//   - specs-in-helper-file: a file named for a supporting role carrying specs,
//     which puts specs where no reader is looking for them.
//   - ginkgo-adapter-file: RunSpecs called from a file not reserved for it.
//   - ginkgo-adapter-missing: a package registering Ginkgo specs where nothing
//     the directory's test binary compiles calls RunSpecs, so none of those
//     specs run.
//   - ginkgo-adapter-duplicate: a second call to RunSpecs in one test binary,
//     which Ginkgo rejects at run time.
//   - suite-file-contents: anything a suite file declares beyond its reason
//     for existing.
//   - misplaced-fixture: a fixture a spec file declares that another file
//     consumes, which a reader of that file finds only by searching.
//   - single-consumer-helper-file: a helper file serving only one file, which
//     separates its fixtures from the only specs that explain them.
//
// # Names, and what <source> means
//
// A pattern is either a shell glob matched against the file name or a <source>
// pattern, never both. The token <source> stands for the base name of a source
// file in the same directory, and what brackets it is matched literally, which
// is what lets "<source>_test.go" mean "named for the source it exercises"
// rather than naming any file in particular. A source split under a GOOS
// or GOARCH file name satisfies it, so platform_test.go is named
// for platform_linux.go and is not read as naming nothing. So does one split
// under _unix, which go/build reads no constraint out of but which is how
// the unix side of a split along the unix build tag is named.
//
// A file matching a helper pattern is required to carry no specs, rather than
// merely permitted to carry none. The two lists therefore divide the allowed
// names rather than overlapping: a name says which of the two a file is,
// and the file has to be it. Helper patterns are consulted first, so
// a helpers.go beside helpers_test.go does not turn the test file into one
// owing specs.
//
// A file carries specs when it calls a Ginkgo container or leaf builder,
// or declares a function — not a method — named for a go test entry point:
// Test, Benchmark, Fuzz, or Example followed by nothing or by a rune
// that is not lower case, which is the rule go test itself applies.
//
// A call is Ginkgo's when the file's imports make it so: a bare Describe
// in a file dot-importing Ginkgo, or a ginkgo.Describe qualified by the name
// the file imports Ginkgo under, declared or renamed. Ginkgo v2's root package
// and the dsl packages re-exporting it count alike. Nothing else does:
// r.Context() is a method call however it is named, and a builder-named
// function that a test package declares for itself or a wrapper package exports
// is not Ginkgo's. The same reading decides a RunSpecs call and a suite hook.
// It is syntactic, so it holds for a file read from disk as it does for one
// the pass holds; the one call it misreads goes through a local declaration
// shadowing a name an import brought in.
//
// # The standalone package
//
// An integration or end-to-end suite commonly lives in a directory of its own
// holding nothing but test files, with any shared harness in a separate package
// it imports. Such a directory has no package under test: the white-box
// and black-box distinction is empty there, and no name can be named
// for the source it exercises. Its files are the standalone kind, named
// for what they exercise, and the standalone category carries their patterns.
// A pattern holding <source> is refused in that category, since it could match
// nothing a standalone file should be called.
//
// A directory is standalone when every source file in it declares nothing.
// A doc.go holding only a package clause keeps it so: saying how to run
// the suite is what such a file is for, and it is what lets go build and go doc
// accept the directory. A file holding only an import does not: a side-effect
// import makes the package do something, and a package that does something
// is a package under test. Whether the package name carries the _test suffix
// is not judged; both spellings compile and both are in wide use.
//
// The other rules hold in a standalone directory as they hold anywhere:
// a helper file carries no specs, the adapter sits in a file reserved
// for it and exists once, and a suite file holds nothing beyond its reason
// for existing.
//
// # The Ginkgo adapter
//
// The adapter is the go test entry point that calls RunSpecs and hands
// the package's registered specs to the test binary. It is one per test binary,
// not one per package: a directory's internal and external test packages
// compile into the same binary, and Ginkgo fails a run where two files call
// RunSpecs.
//
// A suite file — one named by adapter-patterns — is held to what it is for.
// It may declare imports, a go test entry point — func TestXxx(t *testing.T)
// or func TestMain(m *testing.M) — and the suite-level hooks registered as var
// _ = BeforeSuite(...): BeforeSuite and AfterSuite, their Synchronized forms,
// and the ReportBeforeSuite and ReportAfterSuite that bracket a run
// with a report. Nothing else: a helper here is out of the place a reader looks
// for it, and a spec here registers against the very suite the file exists
// to start. The rule holds whether or not the file turns out to carry
// the adapter, because the name is what sends a reader there.
//
// ginkgo-adapter-missing judges the directory rather than the pass.
// The directory is read from disk, because a pass holds one test package
// and the answer can turn on a file in the other; a test file no build compiles
// answers for nothing, since it is not in the binary either. Only the pass
// holding the file a diagnostic lands on reports it, which is what keeps one
// problem from being reported twice. ginkgo-adapter-file judges one file on its
// own. Both apply only where Ginkgo is used: a package of plain go test
// functions has no adapter to place.
//
// # Fixtures
//
// A fixture is a package-level declaration of a test file that specs use but
// that registers nothing itself: a function, a type, a variable, or a constant,
// each name of a grouped declaration on its own. A method goes with its
// receiver's type. Everything else at the top level of a test file is a root —
// a spec, a go test entry point, an init function, a suite hook — and a file
// consumes a fixture when one of its roots reaches it, directly or through
// other fixtures in any file. A file that never names the fixture — does not
// declare it, and refers to it from nothing it declares, a method on another
// file's type included — is listed with the shortest chain of fixtures
// it reaches it through, starting with the one it names, since a search
// of the file would not find it.
//
// A helper name promises that what the file holds is shared, so where a fixture
// lives follows from the files consuming it. One in a spec file that another
// file consumes belongs in that file when it is the only consumer and could
// declare it, and in a helper file otherwise. A helper file whose fixtures all
// serve one file belongs in that file. The helper file is judged whole rather
// than fixture by fixture: a fixture written for one spec and expected to grow
// belongs in a helper file well before its second consumer arrives, while
// a helper file serving one file throughout has no such excuse.
//
// A consumer could declare a fixture when it shares the fixture's package
// and is not a suite file. A black-box file reaching into the white-box package
// through export_test.go cannot hold what it uses, and a suite file holds
// nothing beyond its entry point and hooks, so neither is ever where a fixture
// is sent. A fixture nothing consumes is unused code, which is another linter's
// to report.
//
// Consumers are read from every test file in the directory, both test
// packages and every build, because where a fixture lives is a property
// of the source tree rather than of one platform's build: a fixture serving
// fs_linux_test.go and fs_windows_test.go is shared though no one build
// compiles both. A test file that will not parse could consume anything, so
// beside one nothing is said.
//
// References are resolved without type information. A package-level name
// is declared once, and one clashing with a dot-imported name does not compile,
// so a name that no enclosing scope declares afresh is the fixture. A selector
// names a fixture only when it is qualified by an import of the package under
// test. A composite literal's key is read as a field name unless the literal's
// type is visibly a map, slice, or array, written out or declared as one
// in a file the linter reads, so a fixture keying a literal of an imported map
// type is missed.
//
// # Settings
//
// [Settings] carries a [Category] for each kind, and the names reserved
// for the adapter. Absent keys take the defaults, which allow black-box files
// beside a source and standalone files where there is none:
//
//	whitebox:
//	  allowed: false
//	blackbox:
//	  allowed: true
//	  patterns: ['<source>_test.go', suite_test.go]
//	  helper-patterns: [helpers_test.go, fakes_test.go]
//	standalone:
//	  allowed: true
//	  patterns: ['*_test.go']
//	  helper-patterns: [helpers_test.go, fakes_test.go]
//	adapter-patterns: [suite_test.go]
//
// An absent pattern list takes its default; an explicitly empty one allows
// nothing, which is how a repository says it wants no helper files at all.
// Disallowing the standalone kind is how a repository says every test sits
// beside the code it exercises. A configuration no file could satisfy —
// a malformed glob, a pattern holding a path separator or a glob around
// <source>, a pattern naming the source twice or naming it at all
// for the standalone kind, an allowed category with no name available to it,
// an empty adapter-patterns, no kind allowed — fails the run rather than
// silently matching nothing.
//
// # What this linter does not decide
//
// Which directories the conventions apply to is golangci-lint's to answer, not
// this linter's. A directory of build tooling in package main, or a tree
// that keeps a layout of its own, is excluded through linters.exclusions.paths
// like anything else, so the settings here carry no path list of their own.
// Note also that run.tests, which a plugin cannot read, decides whether this
// linter sees any test files at all.
//
// # Entry points
//
// [New] is the golangci-lint plugin constructor and is what
// github.com/zcayou/go-tools/golangci registers. [NewAnalyzer] builds
// the underlying [analysis.Analyzer] directly and is the entry point for tests
// and for any other go/analysis driver. Both fail on settings no file could
// satisfy.
//
// # Load mode
//
// Every rule is syntactic, so the linter runs under [register.LoadModeSyntax]
// and never forces type checking on a consumer. The fixture rules are no
// exception, and could not be otherwise: type information covers one test
// package and one build, while a fixture's consumers span both test packages
// and every build.
//
// # Suppression
//
// golangci-lint applies the nolint directive centrally, matched on the linter
// name, so a "nolint:testlayout" comment suppresses a finding. This linter does
// not interpret such directives itself. A diagnostic is positioned on the
// package clause, which is the line such a directive has to sit on, except for
// ginkgo-adapter-file, which sits on the RunSpecs call, suite-file-contents,
// which sits on the declaration it is about, and misplaced-fixture, which sits
// on the name of the fixture.
//
// The directive is spelled out rather than written here, because golangci-lint
// reads one out of any comment line — trimming leading slashes and spaces first
// — so writing it would make this paragraph suppress findings on whatever
// follows it.
//
// [analysis.Analyzer]: https://pkg.go.dev/golang.org/x/tools/go/analysis#Analyzer
// [register.LoadModeSyntax]: https://pkg.go.dev/github.com/golangci/plugin-module-register/register#LoadModeSyntax
package testlayout
