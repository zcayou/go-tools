// Package spec holds the ztest rules about what specs do: that they assert
// which error they got, and which member. The ztest linter resolves its
// settings into a [Config] and runs these rules beside the layout rules;
// the settings are documented there.
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
// An assertion that errors.Is found its target, made on the boolean
// it returned, says too little when it fails: the error searched is gone from
// the output.
//
// # Rules
//
// Each diagnostic names the rule that produced it as the first word of its
// message, so a consumer can exclude one rule without silencing ztest:
//
//   - unidentified-error: an assertion that an error occurred, which says
//     nothing about which error, in the specs of an API package.
//   - text-assertion: a positive assertion on an error's text, in a test file
//     that does not pin that rendering.
//   - token-assertion: a positive assertion on an enum's rendering, in a test
//     file that does not pin that enum's tokens.
//   - background-context: context.Background or context.TODO in a test file,
//     where the test framework hands each node a context of its own.
//   - boolean-identity: what errors.Is or errors.As reported, asserted
//     as a boolean where the framework has an assertion that reports the error.
//   - unpinned-rendering, opt-in: an exported error type whose rendering no
//     test file pins.
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
// decision, which in a test file outside the enum's package is zenum's
// token-decision.
//
// An open enum, whose values come from outside the module, needs no exemption.
// A spec showing that a value no member holds survives asserts the value
// converted to the enum, Equal(client.Status("archived")), which is no
// rendering: it asserts the field rather than its text, and the compiler checks
// the type.
//
// # Pinning a rendering
//
// An error's text and an enum's tokens are each asserted once, beside the code
// that renders them. A test file is named for a source beside it when its name
// matches a rendering-patterns entry for that source, under any of the source's
// build-constrained variants, as the layout rules read <source>.
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
// # boolean-identity
//
// Expect(errors.Is(err, target)).To(BeTrue()) fails with "Expected false
// to be true", and the error that was searched is gone from the output. Where
// the framework has an assertion that runs the search and prints the error when
// it fails, an identity boolean asserted as a boolean is reported.
//
// An identity boolean is a call of the standard library's errors.Is
// or errors.As; the second result of errors.AsType, bound in a multi-value
// assignment; a variable declared in a function every write to which is one;
// or the negation of one, or the conjunction or disjunction of two. A module
// function wrapping one is none.
//
// Gomega's MatchError given an error runs errors.Is and prints the actual
// error when it fails, so an errors.Is boolean is judged under Gomega. Gomega
// has no matcher performing errors.As, so an As or AsType boolean is not judged
// there. testify's ErrorIs, NotErrorIs, ErrorAs and NotErrorAs print the chain,
// so all three are judged under testify. A combination is judged when every
// operand is.
//
// Reported, in either polarity, are a Gomega assertion whose actual value
// is a judged identity boolean, under BeTrue, BeFalse, BeTrueBecause
// or BeFalseBecause, or under Equal, BeEquivalentTo or BeIdenticalTo given
// a bool, inside Not, And, Or, SatisfyAll and SatisfyAny as anywhere else;
// and testify's True, False, Equal, EqualValues, Exactly, NotEqual
// and NotEqualValues, and their f forms, given one as a value under test.
// A matcher behind WithTransform is about the transform's result, and is not
// judged. The rule judges every test file, not only an API package's specs:
// the failure output is the same everywhere.
//
// The finding names the function asserted and the assertion to make instead,
// in the polarity the spec asserts: MatchError(target),
// or NotTo(MatchError(target)) where the spec denies the match, under Gomega;
// ErrorIs or NotErrorIs, and ErrorAs or NotErrorAs, under testify. A boolean
// compared with a bool no constant gives could be held to either,
// and the finding names both.
//
// An identity boolean deciding control flow, one inside a WithTransform
// function, and a variable also written from anything else are not reported.
//
// # unpinned-rendering
//
// text-assertion keeps an error's text out of every spec but the one pinning
// it, and nothing makes that one spec assert the text, so a rendering users
// read in logs can change unreviewed. The rule is opt-in, because a rendering
// no spec pins is missing coverage rather than a defect.
//
// A subject is an exported defined type declaring Error() string on itself
// or its pointer, rather than promoting one from a field, where the type
// and the method each sit in a file that is neither a test nor generated,
// in a package other than main. A generated file renders what its generator
// decides, and a cgo file is generated.
//
// A candidate is a test file beside the source declaring a subject's Error
// that a rendering pattern names for that source, under any of its
// build-constrained variants: a file text-assertion already lets assert
// the text. A candidate counts only when its build constraints hold under
// the build tags. Candidates are read from disk, as the pinning check reads its
// sources, so every pass agrees on which exist.
//
// A candidate pins a subject with a positive assertion comparing the text
// of a subject value exactly. The value is an expression whose static type
// is the subject, a pointer to it or an instantiation of it, as a composite
// literal, a typed variable or parameter, a DescribeTable body's parameter,
// a type assertion and a type switch clause's variable listing the subject
// alone all are. The assertion is, in Gomega, Equal(s) on the value's Error(),
// or MatchError(s) on the value, which compares Error() for equality, where s
// has string type and the matcher stands alone or inside And or SatisfyAll,
// never inside Or, SatisfyAny, Not or WithTransform; in testify, Equal
// or Exactly of s and the value's Error(), either way round, or EqualError
// of the value and s, and their f forms. An Error that renders no text,
// panicking by design, is pinned by a positive assertion that a function
// literal whose body is only a call of the value's Error panics: Panic
// or PanicWith standing alone in Gomega, Panics or PanicsWithValue in testify.
// An error-typed value, a prefix or substring matcher, and a negated assertion
// pin nothing.
//
// A subject with no candidate is reported on its Error method's name, naming
// the file the first rendering pattern gives the source declaring it, by every
// pass holding that source. A subject whose candidates none pins is reported
// on the package clause of the anchor: the first candidate, in name order,
// in the external test package, or the first of all when that package holds
// none.
//
// # background-context
//
// Since Go 1.24, testing.T, testing.B and testing.F hand out a context canceled
// just before their cleanups run, and Ginkgo's SpecContext is canceled when
// a spec's NodeTimeout or SpecTimeout expires or the suite is interrupted.
// A test taking context.Background instead opts out of both: a goroutine
// it starts outlives the test, a hang cannot be interrupted, and a timeout
// decorator does nothing. golangci-lint's usetesting reports the same calls
// only inside a function receiving a testing handle, which misses the Ginkgo
// closures, table entries and helpers where most of them live.
//
// Reported is any reference in a test file to context.Background
// or context.TODO, called or not, resolved through type information so every
// import form counts, a package-level initializer included. Two top-level
// functions are exempt, because they run with no test handle to take a context
// from: an Example, and TestMain, whose *testing.M carries none. Work meant
// to outlive the node takes context.WithoutCancel of the node's context.
//
// # Configuration
//
// [Config] carries the API scope, the rendering patterns, each holding <source>
// once, and the build tags a candidate's constraints are read under. A nil
// scope judges every API package: one that is not main and holds no internal
// path element, which a package's own path and name decide with nothing loaded.
// An explicit scope is the module's to answer, and may fail. An empty pattern
// set means no file pins a rendering. A rule in [Config.Disabled] reports
// nothing.
//
// The rules read _test.go files only, so they rely on golangci-lint's
// run.tests, which a plugin cannot read and which is on by default.
//
// # Facts
//
// An enum is known only from the source of the package declaring it, and specs
// meet it in the packages importing that one, which is the way facts flow,
// so the analyzer declares a fact type and needs type information.
// golangci-lint runs an analyzer with facts over every dependency from source,
// then caches the facts; a configuration running govet already pays that cost.
//
// unpinned-rendering adds two facts while it runs, and they are the contract
// between the passes over one directory. The pass over a declaring package
// exports one on each subject, naming the file declaring its Error, and one
// on the package, listing the subjects its own test files pin. golangci-lint
// analyzes a package with in-package tests only as its test variant,
// and the external test package imports that variant, so the external test
// package's pass, which holds none of the package's sources, knows the subjects
// by the first and joins the second with its own pins: an anchor there is its
// to report on. An anchor in the package itself arises only where no external
// candidate exists, and the pass holding it decides alone.
//
// # Positions
//
// A diagnostic is positioned on the matcher or the testify call it is about,
// or, for an assertion whose actual value is the text, on that value.
// unpinned-rendering positions its finding on the subject's Error method name
// or on the anchor's package clause, as its section states.
package spec
