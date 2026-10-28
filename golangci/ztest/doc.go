// Package ztest checks test files: that they sit where a reader looks for them,
// and that their specs say what they verify.
//
// Its rules come in two halves, each in a package of its own that states them
// in full. The layout rules, in [layout], decide what kind of test file may sit
// where, what it may be named, where the Ginkgo adapter goes, and which file
// holds a fixture. The spec rules, in [spec], decide what a spec asserts: which
// error it got rather than that one occurred, through the assertion
// that reports it rather than a boolean; an error's text and an enum's tokens
// only in the test file pinning their rendering, and, opted in, an error's text
// there; and that it runs under the context its framework hands it. Both run
// inside one analyzer named for the linter, under one settings block.
//
// # Rules
//
// Each diagnostic names the rule that produced it as the first word of its
// message, so a consumer can exclude one rule without silencing the linter:
//
//   - test-package, test-file-name, spec-less-test-file, specs-in-helper-file,
//     ginkgo-adapter-file, ginkgo-adapter-missing, ginkgo-adapter-duplicate,
//     suite-file-contents, misplaced-fixture and single-consumer-helper-file,
//     which [layout] states;
//   - unidentified-error, text-assertion, token-assertion, background-context,
//     boolean-identity and the opt-in unpinned-rendering, which [spec] states.
//
// # Settings
//
// [Settings] carries a [Category] for each kind of test file, the names
// reserved for the Ginkgo adapter, the api patterns, the build tags they
// resolve under and test files' constraints are read under, the rendering
// patterns, and the rule selection. Absent keys take the defaults, which allow
// black-box files beside a source and standalone files where there is none,
// judge every API package's specs, and let the test file named for a source pin
// its renderings:
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
//	api: ['./...']
//	build-tags: []
//	rendering-patterns: ['<source>_test.go']
//
// An absent pattern list takes its default; an explicitly empty one allows
// nothing, which is how a repository says it wants no helper files at all,
// or that no file pins a rendering. An empty rendering-patterns cannot stand
// beside unpinned-rendering, which would then require every rendering pinned
// in no file. Disallowing the standalone kind is how a repository says every
// test sits beside the code it exercises.
//
// A pattern is either a shell glob matched against the file name or a <source>
// pattern, as [layout] states. A rendering pattern holds <source> exactly once.
//
// The api patterns narrow the packages whose specs unidentified-error judges,
// resolved from the module root as deadcode's are; a consumer gives zerrors
// and ztest the same list, so the two agree on where identity is owed. Go
// already stops any other module importing an internal package, so no pattern
// makes one API, and a package outside the module is no part of its API.
// Under the default, a package's own path and name decide, and nothing
// is loaded. An explicit list is resolved once for the process, by the first
// pass that asks, under build-tags: golangci-lint's run.build-tags cannot
// be read from a plugin, so a package that exists only under a tag needs
// the tag repeated here.
//
// Every rule but unpinned-rendering is standard, and the default, enable
// and disable keys choose which run, as every linter of this module chooses.
// unpinned-rendering is opt-in because a rendering no spec pins is missing
// coverage, not a defect. The category blocks and adapter-patterns serve
// the layout rules, api serves unidentified-error, build-tags serves
// unidentified-error and unpinned-rendering, which reads a test file's build
// constraints under the tags, and rendering-patterns serves text-assertion,
// token-assertion and unpinned-rendering. Giving one while every rule it serves
// is off configures nothing.
//
// A configuration no run could satisfy fails [New]: settings that do not
// decode, including an unknown key; a rule selection that means nothing,
// or a setting serving no enabled rule; a malformed glob, a pattern holding
// a path separator or a glob around <source>, a pattern naming the source twice
// or naming it at all for the standalone kind; an allowed category with no name
// available to it; an empty adapter-patterns; no kind allowed; an explicitly
// empty api; a rendering pattern holding no <source>; and an explicitly empty
// rendering-patterns while unpinned-rendering runs. What only the module
// can answer fails the run instead: an api pattern matching no API package
// of the module, or an explicit api with no module root to resolve it from.
// Either way the error is a [*ConfigError] naming the setting and the problem.
//
// # What this linter does not decide
//
// Which directories the conventions apply to is golangci-lint's to answer, not
// this linter's. A directory of build tooling in package main, or a tree
// that keeps a layout of its own, is excluded through linters.exclusions.paths
// like anything else, so the settings here carry no path list of their own.
// run.tests, which a plugin cannot read, decides whether this linter sees any
// test files at all.
//
// # Entry points
//
// [New] is the golangci-lint plugin constructor and is what
// github.com/zcayou/go-tools/golangci registers. [NewAnalyzer] builds
// the underlying [analysis.Analyzer] directly and is the entry point for tests
// and for any other go/analysis driver. [Resolve] turns settings into each
// half's configuration. None reads anything from disk.
//
// # Load mode
//
// The linter runs under [register.LoadModeTypesInfo] and declares the spec
// rules' fact type: they resolve every name through type information, and know
// an enum by a fact its declaring package's pass exports. A module enabling
// ztest for its layout rules alone pays for that, though golangci-lint loads
// at the strictest mode any enabled linter asks for, and its standard linters
// are typed and compute facts, so the cost is in practice already paid.
// The layout rules read syntax, and files beyond the build from disk.
//
// # Suppression
//
// golangci-lint applies the nolint directive centrally, matched on the linter
// name, so a "nolint:ztest" comment suppresses a finding. This linter does not
// interpret such directives itself. Where each rule positions its diagnostic
// is stated by the half it belongs to.
//
// The directive is spelled out rather than written here, because golangci-lint
// reads one out of any comment line — trimming leading slashes and spaces first
// — so writing it would make this paragraph suppress findings on whatever
// follows it.
//
// [analysis.Analyzer]: https://pkg.go.dev/golang.org/x/tools/go/analysis#Analyzer
// [register.LoadModeTypesInfo]: https://pkg.go.dev/github.com/golangci/plugin-module-register/register#LoadModeTypesInfo
package ztest
