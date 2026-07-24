// Package deadcode exposes the whole-program dead-code engine
// as a golangci-lint linter.
//
// The analysis itself lives in the engine subpackage. This package
// is the adapter that makes a whole-program result fit go/analysis's
// per-package contract, and it is where the cost of that mismatch is paid.
//
// # How a whole-program result reaches per-package diagnostics
//
// golangci-lint gives a plugin one analyzer, run once per package, whose only
// output channel is [analysis.Pass.Report]. The adapter therefore runs
// the engine once behind a [sync.Once], indexes the findings by the file
// that declares them, and each pass reports the findings belonging to the files
// it holds.
//
// Three consequences follow, and none is avoidable within the plugin API:
//
//   - The engine loads the program itself, in addition to the load
//     golangci-lint already performed, and reads more than golangci-lint does:
//     it needs dependency syntax and type information where golangci-lint can
//     often settle for export data. The cost scales with the whole dependency
//     graph, and the reachability analysis after the load costs more again.
//     It is work the run would not otherwise do, and it does not shrink when
//     golangci-lint is asked for a narrow package pattern, because correct
//     roots need the whole program.
//   - It is work every run does. golangci-lint caches issues per package, keyed
//     on that package and its imports, while dead-code liveness flows the other
//     way — from importers. A cached answer would go stale on an edit
//     that never touches the package it is wrong about, so this linter opts out
//     of that cache entirely and pays the full analysis on every invocation,
//     including one where nothing changed.
//   - The engine's [token.FileSet] is not the pass's, because a second
//     [packages.Load] builds its own. Findings are indexed by the file name
//     the engine's loader opened, unadjusted by any //line directive, because
//     that is the name a pass's [token.File] carries; the byte offset
//     is the same in both, so a position is remapped by offsetting from
//     the file's base. A finding in a file no pass holds is dropped, which
//     is what scopes reporting to the packages the user asked golangci-lint
//     to lint.
//
// # What the plugin cannot do
//
// golangci-lint runs within one module, so a repository of sibling modules
// is analyzed a module at a time and a reference crossing a module boundary
// is not seen as a use. Covering it would mean loading the modules as one
// program, which the plugin has no way to ask golangci-lint for.
//
// That limit is not only one of coverage. As the engine documents, the set
// of loaded packages is an input to interface-participation verdicts:
// an interface declared in a sibling module is a dependency's rather than one
// of the analyzed packages', and a dependency's interface credits its
// implementations unconditionally. A per-module run therefore reports strictly
// fewer method findings than one spanning the repository, rather than a subset
// of the same answer.
//
// Findings in a cgo package do not reach a report. The engine analyzes such
// a package, but cgo rewrites its files and the position a finding carries
// names the rewritten file in the build cache rather than the source the user
// wrote. No pass holds that name, so the finding is dropped. Pairing the source
// file name with the rewritten file's offset is not a fix: the offset
// is meaningless in the other file, so a wrong position would replace a missing
// one.
//
// Nothing bounds how long the analysis takes. go/analysis carries no context,
// golangci-lint offers a plugin none of its own, and its run.timeout
// is a deadline the analysis runner discards and only consults afterwards
// to set an exit code.
//
// # Settings
//
// Because the plugin cannot read golangci-lint's own run configuration, build
// tags and test inclusion are settings on this linter rather than inherited
// from run.build-tags and run.tests.
//
// An absent key and an empty list mean different things. Omitting patterns
// analyzes the whole module; writing patterns: [] names no packages at all
// and is rejected rather than quietly read as the default.
//
// # Suppression
//
// golangci-lint applies the nolint directive centrally, matched on the linter
// name, so a "nolint:deadcode" comment suppresses a finding and the engine does
// not interpret such directives itself. The directive is spelled out rather than
// written here: golangci-lint reads one out of any comment line, trimming
// leading slashes and spaces first, so writing it would suppress findings on
// whatever followed.
//
// [analysis.Pass.Report]: https://pkg.go.dev/golang.org/x/tools/go/analysis#Pass.Report
// [packages.Load]: https://pkg.go.dev/golang.org/x/tools/go/packages#Load
// [sync.Once]: https://pkg.go.dev/sync#Once
// [token.File]: https://pkg.go.dev/go/token#File
// [token.FileSet]: https://pkg.go.dev/go/token#FileSet
package deadcode
