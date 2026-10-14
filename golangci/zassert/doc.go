// Package zassert checks that specs assert which error they got, and which
// member.
//
// An assertion that some error occurred passes on the wrong failure: the call
// under test can fail for a reason the spec never meant, and the spec stays
// green. An assertion on an error's text passes until someone rewords
// the message, and then fails specs that were never about the wording.
// The first says too little and the second says the wrong thing; both stand
// in for asserting the error's identity, its type and its fields, which is what
// a caller branches on. An assertion on an enum's rendered token says the wrong
// thing the same way: it breaks on a renamed token while the behavior holds,
// and passes on the wrong member when String gives two members one token.
//
// # Rules
//
// Each diagnostic names the rule that produced it as the first word of its
// message, so a consumer can exclude one rule without silencing the linter:
//
//   - unidentified-error: an assertion that an error occurred, which says
//     nothing about which error, in the specs of an API package.
//   - text-assertion: a positive assertion on an error's text, in a test file
//     that does not pin that rendering.
//   - token-assertion: a positive assertion on an enum's rendering, in a test
//     file that does not pin that enum's tokens.
//
// # What an assertion is
//
// Every name resolves through the pass's type information. A Gomega call
// is a function or method Gomega declares, in its root package or one beneath
// it: that covers every import form, a Gomega value however it was made,
// and the g an Eventually callback takes. A testify call is a function
// of the assert or require package, or a method of their *Assertions, which
// is also what a testify suite's receiver asserts through. A function or method
// a test package declares for itself is its own, whatever it is named.
//
// A Gomega assertion's actual value is the argument of Expect, Ω,
// ExpectWithOffset, Eventually, EventuallyWithOffset, Consistently
// or ConsistentlyWithOffset, past the offset. Eventually and Consistently take
// a context ahead of the actual value, so the argument after the first
// is the actual value when the first's type implements context.Context.
// The actual value is never a method chained onto the assertion:
// Expect(f()).Error() selects f's error result, which is an error and not its
// text.
//
// What a variable holds is read from every write the package's files make
// to it. A write through a pointer or a field is not one.
//
// A matcher's polarity is what decides whether it asserts or denies.
// To and Should are positive, NotTo, ToNot and ShouldNot negative, and every
// Not enclosing the matcher flips it, inside And, Or, SatisfyAll and SatisfyAny
// as anywhere else. A matcher held outside any assertion, as a table entry
// hands one on, has only the Nots enclosing it to go by. WithTransform
// is transparent when its transform is one of the strings functions deriving
// a string from a string, such as strings.ToLower, and opaque otherwise:
// the matcher it holds sees whatever the transform made of the value,
// and is not judged for text. A negated check is never reported.
//
// Of a testify assertion, only the values under test it is given are judged,
// never the failure message and its arguments that follow them: assert.Equal(t,
// want, got, err.Error()) asserts nothing of the text.
//
// A value is an error when its type implements error, so err from x, err := f()
// is one when f returns an error.
//
// # unidentified-error
//
// Reported: Gomega's HaveOccurred in positive polarity, Succeed in negative
// polarity, and BeNil in negative polarity on an error; testify's Error
// and Errorf, and NotNil and NotNilf on an error. A BeNil or NotNil whose
// actual value is not an error is not judged, and a BeNil behind
// a WithTransform is about the transform's result, not the error. The assertion
// is wrong either way: it is the only check, or it is redundant before one
// that identifies the error. Assert the error's type and fields instead —
// BeAssignableToTypeOf and HaveField, MatchError of the expected error,
// errors.AsType, or testify's ErrorIs and ErrorAs.
//
// The rule judges only the specs of API packages: not main, no internal path
// element, and matched by the api setting. An internal package may return raw
// errors that its API caller wraps, so its specs have nothing to identify.
// A test package answers for the package it tests, so the external foo_test
// package is judged with foo.
//
// # text-assertion
//
// An error's text is Error() called on an error; an fmt.Sprint, Sprintf
// or Sprintln of an error; a strings transformation, slice or concatenation
// of text; or a variable every write to which is text, as msg is after
// msg := err.Error().
//
// Text assertions are, in Gomega:
//
//   - MatchError given a string literal or a concatenation holding one,
//     an fmt.Sprint, Sprintf or Sprintln call, a strings transformation,
//     an error's text, or a string matcher: ContainSubstring, HavePrefix,
//     HaveSuffix, MatchRegexp, Equal or BeEquivalentTo, alone, composed,
//     or behind a transparent WithTransform. MatchError applies a matcher
//     to the error's text, so every one of these compares text;
//   - MatchError given an error the spec builds with errors.New or fmt.Errorf.
//     MatchError compares an error value with reflect.DeepEqual, which for such
//     an error compares its text, so the finding says to match the error's
//     identity or type instead;
//   - HaveField("Error()", …);
//   - an assertion whose actual value is an error's text.
//
// And in testify: EqualError and ErrorContains, their f forms, and any other
// assertion given an error's text as a value under test, except the Not
// assertions.
//
// Two kinds of text assertion are exempt. A negated one proves a value absent
// from a message, such as a secret, and only a text assertion can prove that.
// One on a cause asserts a rendering the raise site made: an idiom that keeps
// the values a rule compared in the cause, rather than in fields, asserts them
// there, and the cause has no type of its own to pin it. A cause
// is errors.Unwrap of an error, or a variable every write to which is one, each
// made in the function declaring the variable. A variable also written anything
// else, or written from a function literal, which may run at any time, is no
// cause.
//
// A string variable given to MatchError, such as a DescribeTable parameter,
// is not judged unless its writes show it to be an error's text.
//
// # token-assertion
//
// An enum's rendering is what zenum's token-decision takes as one: v.String()
// with the enum's own String method, string(v) of a string enum, or fmt.Sprint
// of an enum value. A strings transformation or a slice of a rendering is one
// too, and so is a variable every write to which is one.
//
// Token assertions are, in Gomega, an assertion whose actual value
// is a rendering, under any matcher, and a HaveField whose path ends
// in String() on an enum, such as HaveField("State.String()", "pending");
// in testify, any assertion given a rendering as a value under test, except
// the Not assertions. A negated one is exempt, as a negated text assertion is.
// The spec asserts the member instead: Equal(StatePending).
//
// An enum is what the module's shared definition says it is, read from
// the syntax of the package declaring it: a defined type with typed constants,
// not a flag set, a quantity or a generic type. The pass over that package
// exports a fact on the type, and the specs of every package importing it read
// the fact, so a spec in an external test package knows an enum it can see only
// the types of. Only an enum declared in the spec's own module is judged,
// the module whose go.mod sits nearest above both; time.Month and every other
// foreign enum is its owner's. An enum declared in a cgo file is not judged,
// since its file sits in the build cache.
//
// Only assertions are judged. A spec comparing a rendering by hand makes a text
// decision, and decisions in test files are no rule's. zenum's open-enums
// is zenum's own: zassert reads no other linter's settings, so a spec asserting
// an open type's wire token goes through a nolint directive or an exclusion.
//
// # Pinning a rendering
//
// An error's text and an enum's tokens are each asserted once, beside the code
// that renders them. A test file is named for a source beside it when its name
// matches a rendering-patterns entry for that source, under any of the source's
// build-constrained variants, as testlayout reads <source>.
//
// A test file pins an error's text when a source it is named for declares
// a method Error() string. So errors_test.go may assert the text errors.go
// renders, and every other spec asserts types and fields. The source is read
// from disk, because the external test package's pass holds none
// of the package's sources; a source that will not parse could declare
// anything, so the files named for it are left alone.
//
// A test file pins an enum's tokens when a source it is named for is the file
// declaring the enum's type, or the file declaring its String, as the enum's
// fact records. So kind_test.go pins the tokens of a Kind declared in kind.go,
// whether its String is written there or generated into kind_string.go.
//
// # Settings
//
// [Settings] carries the api patterns, the build tags they resolve under,
// and the rendering patterns. Absent keys take the defaults:
//
//	api: ['./...']
//	build-tags: []
//	rendering-patterns: ['<source>_test.go']
//
// The api patterns narrow the packages whose specs unidentified-error judges,
// resolved from the module root as deadcode's are; a consumer gives zerrors
// and zassert the same list, so the two agree on where identity is owed. Go
// already stops any other module importing an internal package, so no pattern
// makes one API, and a package outside the module is no part of its API.
// Under the default, a package's own path and name decide, and nothing
// is loaded. An explicit list is resolved once for the process, by the first
// pass that asks, under build-tags: golangci-lint's run.build-tags cannot
// be read from a plugin, so a package that exists only under a tag needs
// the tag repeated here.
//
// A rendering pattern is in testlayout's grammar and holds <source> exactly
// once. An explicitly empty list means no file pins a rendering.
//
// A configuration no run could satisfy fails [New]: settings that do not
// decode, including an unknown key; an explicitly empty api; a rendering
// pattern holding no <source>, holding it twice, wrapped in a glob, or holding
// a path separator. What only the module can answer fails the run instead:
// an api pattern matching no API package of the module, or an explicit api
// with no module root to resolve it from. Either way the error
// is a [*ConfigError] naming the setting and the problem.
//
// zassert reads _test.go files only, so it relies on golangci-lint's run.tests,
// which a plugin cannot read and which is on by default.
//
// # Entry points
//
// [New] is the golangci-lint plugin constructor and is what
// github.com/zcayou/go-tools/golangci registers. [NewAnalyzer] builds
// the underlying [analysis.Analyzer] directly and is the entry point for tests
// and for any other go/analysis driver. Neither reads anything from disk.
//
// # Load mode
//
// The linter runs under [register.LoadModeTypesInfo] and declares a fact type.
// An enum is known only from the source of the package declaring it, and specs
// meet it in the packages importing that one, which is the way facts flow.
// golangci-lint runs an analyzer with facts over every dependency from source,
// then caches the facts; a configuration running govet already pays that cost.
//
// # Suppression
//
// golangci-lint applies the nolint directive centrally, matched on the linter
// name, so a "nolint:zassert" comment suppresses a finding. This linter does
// not interpret such directives itself. A diagnostic is positioned on
// the matcher or the testify call it is about, or, for an assertion whose
// actual value is the text, on that value.
//
// [analysis.Analyzer]: https://pkg.go.dev/golang.org/x/tools/go/analysis#Analyzer
// [register.LoadModeTypesInfo]: https://pkg.go.dev/github.com/golangci/plugin-module-register/register#LoadModeTypesInfo
package zassert
