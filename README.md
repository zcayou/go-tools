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

Each linter's package doc — [testlayout], [deadcode], [zlines] — is the full
reference for its rules; the sections below cover configuration.

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
          api-exempt: [method]     # default when api is set
          api-generics: instantiated  # default when api is set; skip roots none of it
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

A sealed interface — one declaring an unexported method, so that no other
package can implement it — is weighed against the declared surface. Nothing in
the module converts a handle to it when the factory hands back the concrete
type, because a consumer does that; and no exemption reaches the resulting
verdict, since the exempt kinds cover the exported surface while the sealing
method is unexported by construction. So an exported sealed interface in an
`api` package credits its implementations, the way a dependency's interface
does. It takes `api`: sealing on its own credits nothing. A generic sealed
interface is weighed per instantiation the program builds, and one the program
never instantiates credits nothing — the implementation and the contract are
then reported as a pair.

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
type argument does decide is which of its own methods become runtime types, so a
production method that only a test's type argument selects is credited rather
than reported. That is the one thing this setting gives up; `skip` is how to
decline it.

A generic the program never instantiates has nothing to monomorphize, so its
body is walked instead: the concrete functions it calls are rooted directly,
because it calls them whatever a consumer instantiates it with. Where that walk
meets a call it cannot resolve — through an interface or a function value — the
generic is reported as `unmeasured exported generic` rather than letting what
lies past it be called dead. That verdict is about the analysis, not the
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

## Usage

Module plugins are compiled in, so a consumer builds its own binary. Add
`.custom-gcl.yml` at the repository root:

```yaml
version: v2.12.2
name: custom-gcl
destination: ./bin
plugins:
  - module: 'github.com/zcayou/go-tools'
    import: 'github.com/zcayou/go-tools/golangci'
    version: v0.1.0   # a released tag; see the repository's tags for the current one
```

Install the matching bootstrap binary and build (requires Go 1.26+):

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
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
the deadcode engine's fixtures are whole modules, and module-zip creation drops
directories containing a `go.mod`, so a copy fetched through a Go proxy is
missing them.

## License

Copyright 2026 Zachary Cayou. [Apache License 2.0](LICENSE).

[testlayout]: https://pkg.go.dev/github.com/zcayou/go-tools/golangci/testlayout
[deadcode]: https://pkg.go.dev/github.com/zcayou/go-tools/golangci/deadcode
[zlines]: https://pkg.go.dev/github.com/zcayou/go-tools/golangci/zlines
