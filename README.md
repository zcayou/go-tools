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

Sorts every test file into one of two kinds by the package it declares —
white-box files declare the package under test, black-box files declare the
external `<pkg>_test` package — and checks which kinds are allowed and what
each may be named. Patterns are globs, except that `<source>` stands for the
base name of a source file in the same directory. The defaults allow black-box
files only:

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
          adapter-patterns: [suite_test.go]
```

Exclude directories with their own layout through `linters.exclusions.paths`.
Don't enable golangci-lint's built-in `testpackage` alongside this linter —
they overlap under the default settings.

### zlines

Three rules, all fixable with `--fix`: `signature-wrap` (a signature spread
over lines that would fit on one), `body-collapse` (a body written on the line
its signature ends on), and `comment-wrap` (comment paragraphs not filled to
the limit). Fixes are only offered when gofmt agrees with the result, so
running the formatter afterwards cannot undo them.

```yaml
linters:
  settings:
    custom:
      zlines:
        type: module
        settings:
          line-length: 160        # default 120, matching lll
          comment-length: 80      # default 80
          signature-wrap: false   # default true
          body-collapse: false    # default true
          comment-wrap: false     # default true
          comment-exempt: ['+kubebuilder']
```

Set `line-length` to the limit the repository already enforces; it governs
`signature-wrap` only. `comment-exempt` lists prefixes marking comment lines
meant for a tool rather than a reader — those lines stay where they are.

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
          tests: true              # default; matches run.tests
          api: ['./pkg/...']       # package patterns whose exported surface consumers reach
          api-exempt: [method]     # default when api is set
```

Two settings that need care:

- Set `issues.uniq-by-line: false` — the default `true` silently discards
  findings, because deadcode reports more than one verdict per declaration on
  purpose.
- To suppress findings in test-support packages, exclude their *paths* rather
  than narrowing `patterns`. Narrowing `patterns` shrinks what gets loaded,
  which invents dead code: a package dropped from the analysis stops being a
  use of anything.

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
