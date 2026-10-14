# tools

[![CI](https://github.com/zcayou/go-tools/actions/workflows/ci.yml/badge.svg)](https://github.com/zcayou/go-tools/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/zcayou/go-tools.svg)](https://pkg.go.dev/github.com/zcayou/go-tools)

Standalone tooling. What it holds today is code-hygiene linters, delivered as
[golangci-lint module plugins](https://golangci-lint.run/docs/plugins/module-plugins/):
a consumer compiles them into its own golangci-lint binary and configures them
like any other linter, so `//nolint`, `linters.exclusions`, output formats, and
existing CI wiring apply unchanged.

## Linters

| Name | What it checks | Fixes |
|---|---|---|
| `testlayout` | Test files follow the repository's test-suite layout conventions. | |
| `deadcode` | Code that nothing can reach, by whole-program call-graph analysis. | |
| `zlines` | Line breaks a declaration or comment should not have, and line breaks it should. | `--fix` |
| `zerrors` | Errors a caller cannot tell apart, decisions on error text, and error fields that should be enums. | |
| `zenum` | Enums that are not ordinal, unnamed, ambiguously named, or made by conversion. | |
| `zassert` | Specs that assert an error occurred without saying which, or assert an error's text or an enum's token outside the spec that pins it. | |

Each linter's package doc — [testlayout], [deadcode], [zlines], [zerrors],
[zenum], [zassert] — is the full reference for its rules; the sections below
cover configuration.

### testlayout

Sorts every test file into one of three kinds and checks which kinds are
allowed and what each may be named. Beside a source, the package a file
declares decides — white-box files declare the package under test, black-box
files declare the external `<pkg>_test` package. In a directory holding no
source to test, such as an integration suite, a file is standalone and named
for what it exercises. Patterns are globs, except that `<source>` stands for
the base name of a source file in the same directory. The defaults allow
black-box files beside a source and standalone files where there is none:

```yaml
linters:
  settings:
    custom:
      testlayout:
        type: module
        settings:
          whitebox:
            allowed: false
          blackbox:
            allowed: true
            patterns: ['<source>_test.go', suite_test.go]
            helper-patterns: [helpers_test.go, fakes_test.go]
          standalone:
            allowed: true
            patterns: ['*_test.go']
            helper-patterns: [helpers_test.go, fakes_test.go]
          adapter-patterns: [suite_test.go]
```

A fixture — a package-level declaration specs use — lives in the one file
consuming it, or in a helper file when more than one does: `misplaced-fixture`
reports one a spec file declares for another file, and
`single-consumer-helper-file` a helper file serving only one.

Exclude directories with their own layout through `linters.exclusions.paths`.
Don't enable golangci-lint's built-in `testpackage` alongside this linter —
they overlap under the default settings.

### zlines

Three rules, all fixable with `--fix`: `signature-wrap` (a signature spread
over lines that would fit on one), `body-collapse` (a body written on the line
its signature ends on), and `comment-wrap` (a comment paragraph breaking a
line outside the band its settings draw). Fixes are only offered when gofmt
agrees with the result, so running the formatter afterwards cannot undo them.

```yaml
linters:
  settings:
    custom:
      zlines:
        type: module
        settings:
          line-length: 160        # default 120, matching lll
          comment-length: 80      # default 80
          comment-min-length: 70  # default: 10 under comment-length
          comment-overrun: 10     # default 10
          signature-wrap: false   # default true
          body-collapse: false    # default true
          comment-wrap: false     # default true
          comment-exempt: ['+kubebuilder']
```

Set `line-length` to the limit the repository already enforces; it governs
`signature-wrap` only. `comment-exempt` lists prefixes marking comment lines
meant for a tool rather than a reader — those lines stay where they are.

Comment lines may stop anywhere between `comment-min-length` and
`comment-length`, so editing one word mid-paragraph does not rewrap every line
below it; a line short of the minimum is reported only when the next word
would still have fit, and a break after a joining word ("the", "of") is
reported at any width. Setting the minimum equal to the length restores the
exact greedy fill.

A paragraph of one line may run `comment-overrun` columns past
`comment-length` rather than shed its last word onto a line of its own. A
longer paragraph never may: it can move an earlier break inside the band to
give its last line company, and a single line has no break to move. Set the
overrun to 0 for a hard limit.

### deadcode

A whole-program analysis. What it loads is governed by its own `patterns`
setting, not the package arguments golangci-lint was invoked with, and it
re-runs on every invocation — golangci-lint's cache never spares the work.
Plan on a separate CI-only invocation rather than enabling it on save. A
repository of sibling modules is analyzed a module at a time; see the
[deadcode] package doc for what that changes.

```yaml
linters:
  settings:
    custom:
      deadcode:
        type: module
        settings:
          patterns: ['./...']      # absent means ./... from the module root
          build-tags: [integration]
          tests: true              # default; matches run.tests, and reports the test-only family
          api: ['./pkg/...']       # package patterns whose exported surface consumers reach
          api-exempt: [method]     # default when api is set; [] exempts nothing
          api-generics: instantiated  # default when api is set; skip roots none of it
          api-test-consumers: true # api packages' tests stand in for their consumers
          enum-names: true   # an enum's String is its published name
          test-facing: ['./plugins/test/...']  # packages whose intended consumers are tests
          roots: ['tools/*.go']    # entry-point files the load cannot reach
```

With `tests: true`, production code that only test code keeps alive is
reported as a finding family of its own: every verdict has a `test-only` form
(`test-only unreachable func: Compute`, `test-only unused exported method:
Runtime.Program`, …), drawn by a declaration the analysis reports once
test-origin evidence is set aside. Code declared in `_test.go` files never
draws one — the family is about production declarations whose only consumers
are their tests. Silence the whole family with one exclusion on the prefix:

```yaml
linters:
  exclusions:
    rules:
      - linters: [deadcode]
        text: '^test-only '
```

A re-export on the declared surface is judged as the declaration it renames.
An exported `type X = p.Y` (instantiated or not) or `const X = p.Y` of the same
type in an `api` package, naming a declaration another `api` package makes,
draws no verdict of its own: every reference to it counts for `p.Y`, following
a chain of copies to its end, and its own right-hand side does not. So a facade
renaming the module's enum is not reported name by name because the
module's own code spells the owner, and a dead `p.Y` cannot hide behind a copy
of it — it is reported where it is declared, in its `test-only` form when only
a test spells any of its names. A conversion, a second name within one package,
and a copy of something the surface does not declare — a package it leaves out,
a dependency — are declarations of their own, since the copy is then how that
declaration reaches consumers.

A function cannot be aliased, so its copy is a forwarder: an exported function
in an `api` package whose body is one call to another `api` package's function
— returned, or made bare when there are no results — passing its own type
parameters and parameters, each once and in order, with a signature identical
to the callee's, constraints included. Like an alias it draws no verdict,
`unreachable func` included, whatever name it gives, and a reference to it
counts for the callee. A function that reorders its arguments, adds a
statement, narrows a constraint, or forwards within its own package or off the
surface is a declaration of its own.

A sealed interface — one declaring an unexported method, so that no other
package can implement it — is weighed against the declared surface. Nothing in
the module converts a handle to it when the factory hands back the concrete
type, because a consumer does that; and no exemption reaches the resulting
verdict, since the exempt kinds cover the exported surface while the sealing
method is unexported by construction. So an exported sealed interface in an
`api` package credits its implementations, the way a dependency's interface
does, along with what follows from a consumer holding one: what the
implementations call stays live, and an assertion from the sealed interface to
another finds them. It takes `api`: sealing on its own credits nothing.
A generic sealed interface is weighed per instantiation the program builds, and
one the program never instantiates credits nothing — the implementation and the
contract are then reported as a pair. Under the default `api-generics:
instantiated` a test's instantiation counts in the `test-only` family's masked
view as it does in the full one, for the reason it roots the surface there
(below); under `skip` the masked view counts no test's instantiation.

Where an interface is declared decides nothing about how it is weighed; whether
the run loads it does. Binding a type to an interface credits that type's method
only while something selects the interface method — otherwise a dead interface
would launder every implementation behind it — and an interface declared in a
`_test.go` file is subject to that rule like any other, because every call site
it has is one the run loaded. Only a dependency's interface, whose call sites
are invisible, credits unconditionally. So a fixture registering a type against
a contract it never exercises draws both halves, `unreachable func` on the
implementation and `unused interface method` on the contract, and the pair is
the finding: the contract is what wants deleting or asserting on.

`api-generics` decides how the declared surface's generic declarations are
rooted, and it matters most for the libraries that need `api` most. Rapid Type
Analysis roots concrete functions, so a generic declaration cannot be one: an
uninstantiated body has no instance to root at, and a type parameter reaching
RTA as a runtime type panics it. A library whose public API is generic would
therefore root almost nothing, and the whole helper layer beneath that API would
be reported as unreachable — a failure to root, wearing the shape of a liveness
verdict.

The default, `instantiated`, roots the concrete instantiations the program
builds of that surface instead. `skip` is the older behavior, kept for a run
that wants nothing rooted it cannot root directly. Those are ordinary functions, and reachability reads them like
any other. The type arguments are not evidence about the API: an instantiation
calls the same declarations whichever a consumer picks, and the reachable set
comes from the body. Instantiations therefore count wherever they were written,
test files included — a library's own tests are usually the only code
instantiating its public generics, and refusing them would leave the surface
unrooted in precisely the masked view the `test-only` family asks about. What a
type argument does decide is which of its own methods become runtime types, and
which implementations a sealed generic contract matches, so a production method
that only a test's type argument selects is credited rather than reported. That
is the one thing this setting gives up; `skip` is how to decline it.

A generic the program never instantiates has nothing to monomorphize, so its
body is walked instead: the concrete functions it calls are rooted directly,
because it calls them whatever a consumer instantiates it with. The same walk
covers a method the analysis credits — a sealed interface's implementation,
say — on a generic type nothing instantiates. Where that walk meets a call it
cannot resolve — through an interface or a function value — the generic is
reported as `unmeasured generic` rather than letting what lies past it be
called dead. That verdict is about the analysis, not the
declaration, and no `api-exempt` kind covers it. It is reported by default,
because an unfollowed edge is worth knowing about; silence it, if you must, the
same way as any other rule:

```yaml
linters:
  exclusions:
    rules:
      - linters: [deadcode]
        text: '^unmeasured '
```

`roots` is the stronger answer where you can pay for it, and the two compose: a
root program supplies the instantiation a real consumer would, and anything it
does not reach stays reported.

`test-facing` declares packages whose intended consumers are tests — a test
plugin that looks, feels, and compiles like a production plugin, kept alive by
nothing but the tests that exercise it, which is its job. A declared package
stops drawing the `test-only` family on its own surface (dead code inside it
still reports with plain verdicts), while a production declaration only it
keeps alive draws the family completely. It requires `tests: true`, and a
package cannot be declared both `api` and `test-facing`.

`api-test-consumers` counts an `api` package's tests as stand-ins for the
consumers the run cannot see. Without it, `tests: true` sets them aside with
every other test, so a public adapter only the tests exercise, or a seam only
they call, draws the `test-only` family. With it, test evidence aimed at the
declared surface counts in the masked view as production evidence would, as
far as a consumer could have written it: a reference to an exported declaration
of an `api` package; a selection of an exported method through an exported
`api` type (promoted ones included); a conversion, concrete-to-interface or
interface-to-interface, with an exported `api` type at one end and nothing at
either end a consumer cannot name; and, unless `api-generics: skip`, an
instantiation of an `api` generic with type arguments a consumer can name. A
consumer can name exported `api` types, its own types (a test's), and its
dependencies' types, so a test handing an `api` function a production type the
`api` does not export stays test evidence. An interface a test declares over a
public type credits what is bound to it, the way a consumer's own interface
would. Test evidence aimed anywhere else stays masked, so production code off
the surface that only tests keep alive still draws the family, and an exported
`api` declaration nothing references, tests included, is still reported. It
requires `api` and `tests: true`.

`enum-names` credits an enum's published name. A closed
enum, as `internal/enum` defines it for every linter here — a
non-generic named type whose own package declares typed constants of it, flag
sets and quantities excepted — is an enum, and a
`String() string` method on a value receiver is its name: it counts as used,
and what it calls stays live, whether or not production ever renders a member.
That is a repository's convention — one name per member, rendered or not —
rather than evidence the program carries, so it is off by default. Other
methods of the type, and a `String` on a type with no declared constant, are
judged as ever.

`roots` names entry-point files the loader cannot reach: conventionally
`package main` generators behind `//go:build ignore`, run with `go run`, where
the tag is what lets several such programs share one directory — which is also
why `build-tags: [ignore]` cannot admit them without making the siblings
collide. Each named file (paths or globs, relative to the module root) is
loaded as its own single-file main program: its call edges and references count
as production evidence in both views, and nothing in it is ever reported.

Two settings that need care:

- Set `issues.uniq-by-line: false` — the default `true` silently discards
  findings, because deadcode reports more than one verdict per declaration on
  purpose.
- Never suppress test-support packages by narrowing `patterns`. Narrowing
  `patterns` shrinks what gets loaded, which invents dead code: a package
  dropped from the analysis stops being a use of anything. Declare the package
  `test-facing` instead.

### zerrors

Holds a module's errors to two principles: an error that leaves the API can be
told apart by its caller, through a type or sentinel it can name, and nothing
decides on an error's text. Like deadcode it analyzes the whole module once per
invocation from its own typed load — module source, dependencies from export
data, no tests — so it re-runs every time; zenum shares that load when both are
enabled with the same build tags.

```yaml
linters:
  settings:
    custom:
      zerrors:
        type: module
        settings:
          api: ['./pkg/...']        # absent means ./...; main and internal packages are never API
          trusted: ['example.com/framework/...']  # packages whose errors count as structured
          protocols:                # interface methods whose implementations may return raw errors
            - {method: example.com/framework/lane.Acquirer.Acquire, reason: the lane classifies them}
          sentinels: true           # default; false requires error types rather than variables
          base: example.com/framework/contract.Classification  # every error type carries it
          base-packages: ['./pkg/...', './internal/...']        # absent means ./...; where base applies
          build-tags: [integration]
```

`unstructured-return` judges what a caller outside the module calls: exported
functions and methods of API packages, implementations of their exported
interfaces, and implementations the module hands to a `trusted` framework.
`text-decision` reports a branch on `Error()` or on an error type's `string`
field, anywhere. `literal-field` reports an error type's `string` field that
only ever holds constants, which wants an enum type, or, when it only ever holds
one value, carries no information and wants removing. `missing-base`, only
while `base` is set, reports an error type that does not carry it, in the
packages `base-packages` matches: code that stays outside the framework on
purpose is left out there rather than through exclusions. Unlike `api`, its
patterns match `internal` and `main` packages too.

A `trusted`, `protocols` or `base` entry that matches nothing, and an `api` or
`base-packages` pattern matching no package it could mean, fail the run rather
than meaning nothing.

### zenum

Holds enums — a defined, non-generic integer or string type whose own package
declares constants of it — to being ordinal, naming each member once through a
readable `String`, keeping zero out of the members, and being the only way to
make their values. A type whose constants are declared by a `<<` shift (a flag
set) or by multiplying another of its constants (a quantity) is no enum, and an
unexported constant that closes an iota run and serves as an array length or
ordering bound (a count sentinel) is no member. It shares zerrors's whole-module
load.

```yaml
linters:
  settings:
    custom:
      zenum:
        type: module
        settings:
          open-enums:               # values come from outside the module: no zenum rule applies
            - {type: example.com/m/wire.ID, reason: IDs come from the wire}
          zero-defaults:            # zero is the documented default member
            - {type: example.com/m/sync.Strategy, reason: zero means merge}
          related-enums:            # tokens overlap because they classify different things
            - {first: example.com/m/a.State, second: example.com/m/b.Phase, reason: phases mirror states}
          build-tags: [integration]
```

The rules are `string-enum`, `unnamed`, `unreadable-name`,
`ambiguous-name`, `zero-member`, `restated`, `token-decision`,
`forged-conversion` and `numeric-verb`. A `String` is read rather than run, in
the forms people write and the three layouts `stringer` generates; a member's
token is a string reached for its value specifically — read from a table by the
receiver, or inside a branch an equality on the receiver entered — and a
fallback gives none. Each entry
is a property of the type, applied wherever it is used, and needs a reason; an
entry that suppresses nothing fails the run. A one-off goes through
`//nolint:zenum` or an exclusion.

### zassert

Holds Ginkgo/Gomega and testify specs to asserting which error they got, and
which enum member. It judges test files only, so it needs golangci-lint's
`run.tests`, the default. It is typed: an enum is known by a fact the pass over
its declaring package exports, so golangci-lint analyzes dependencies from
source and caches the facts, as it already does for govet.

```yaml
linters:
  settings:
    custom:
      zassert:
        type: module
        settings:
          api: ['./pkg/...']                    # give zerrors and zassert the same list
          build-tags: [integration]             # the tags the api patterns resolve under
          rendering-patterns: ['<source>_test.go']  # default; [] means no file pins a rendering
```

`unidentified-error` reports `HaveOccurred()`, a negated `Succeed()`, a negated
`BeNil()` on an error, and testify's `Error` and `NotNil` on an error, in the
specs of API packages. `text-assertion` reports an assertion on an error's
text — `MatchError` given text or an error built with `errors.New` or
`fmt.Errorf`, `Expect(err.Error())` or a variable holding it, `EqualError`,
`ErrorContains` and the like — except in the spec file that pins a rendering:
one a `rendering-patterns` entry names for a source declaring `Error() string`.
`token-assertion` reports an assertion on an enum's rendering —
`Expect(s.String())`, `string(s)` of a string enum, `fmt.Sprint(s)`,
`HaveField("State.String()", …)`, testify's `Equal` of one — except in the spec
file a `rendering-patterns` entry names for the file declaring the enum or its
`String`. Only enums declared in the module are judged. A negated assertion is
exempt from both, and so is a text assertion on `errors.Unwrap(err)`.

## Usage

Module plugins are compiled in, so a consumer builds its own binary. Add
`.custom-gcl.yml` at the repository root:

```yaml
version: v2.13.2
name: custom-gcl
destination: ./bin
plugins:
  - module: 'github.com/zcayou/go-tools'
    import: 'github.com/zcayou/go-tools/golangci'
    version: v0.1.0   # a released tag; see the repository's tags for the current one
```

Install the matching bootstrap binary and build (requires Go 1.27+):

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
golangci-lint custom
```

Enable the linters in `.golangci.yml`:

```yaml
version: "2"
linters:
  enable:
    - testlayout
  settings:
    custom:
      testlayout:
        type: module
        description: Checks that test files follow the repository's test-suite layout conventions.
        original-url: github.com/zcayou/go-tools
```

Run `./bin/custom-gcl run` in place of `golangci-lint run`. Suppress a finding
with `//nolint:<linter>`, or exclude one rule of a linter by matching the token
that opens every message:

```yaml
linters:
  exclusions:
    rules:
      - linters: [testlayout]
        text: '^spec-less-test-file:'
```

`testlayout` and `zlines` open each message with the rule name; `deadcode`
opens with the verdict (`unreachable func:`, `unused exported func:`, and so
on).

**Clear the cache after every rebuild.** Rebuilding the binary does not
invalidate golangci-lint's cache, so a stale build replays its previous
findings — and under `--fix`, applies the previous build's edits. Run
`./bin/custom-gcl cache clean` after `golangci-lint custom`.

## Development

```
make help      list targets
make test      run the suites
make lint      lint this module, dogfooding its own linters
make verify    build, test, lint — the same set CI runs
```

`.custom-gcl.yml`'s `version` is the single source of truth for which
golangci-lint this module targets; the Makefile reads it to pin the bootstrap
binary, so the two cannot drift.

Suites are Ginkgo/Gomega, with analyzer fixtures under
`golangci/<name>/testdata/` driven by `analysistest`. Run them from a checkout:
the fixtures of the whole-module linters and their engines are whole modules,
and module-zip creation drops directories containing a `go.mod`, so a copy
fetched through a Go proxy is missing them.

## License

Copyright 2026 Zachary Cayou. [Apache License 2.0](LICENSE).

[testlayout]: https://pkg.go.dev/github.com/zcayou/go-tools/golangci/testlayout
[deadcode]: https://pkg.go.dev/github.com/zcayou/go-tools/golangci/deadcode
[zlines]: https://pkg.go.dev/github.com/zcayou/go-tools/golangci/zlines
[zerrors]: https://pkg.go.dev/github.com/zcayou/go-tools/golangci/zerrors
[zenum]: https://pkg.go.dev/github.com/zcayou/go-tools/golangci/zenum
[zassert]: https://pkg.go.dev/github.com/zcayou/go-tools/golangci/zassert
