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
// # Rules beside the verdicts
//
// The verdicts are the reachability analysis itself, reported under their own
// tokens, and are not selectable: a module that wants none of them does not
// enable deadcode. Beside them sit rules, which judge something further
// and which the default, enable and disable keys choose among, as every linter
// of this module chooses. A selection enabling none still leaves the verdicts.
// There are two rules:
//
//   - write-only-field, standard: a field of a named struct type that module
//     code writes and nothing reads. It is state with no effect.
//   - inert-field, opt-in: an exported field of a struct the module takes from
//     its caller that no live module code reads. Such a field type-checks
//     and round-trips, the caller sets it, and nothing happens.
//
// write-only-field judges every non-blank, non-embedded field of a named struct
// type declared outside a generated file, test files included while tests
// is on, and a generic type's field once for all its instantiations. A field
// of an anonymous struct type is not judged: identical unnamed struct types
// assign to each other, so no one declaration owns the field. A field
// with a write and no read is reported, and one nothing selects at all is left
// to unused. With api declared, the exported fields of a struct type a consumer
// can hold are not judged, since the consumer may read them. It holds
// the exported types of the api packages, which it names, and what the API's
// signatures and variables reach in either direction, as the polarity walk
// below crosses them, together with what lies beneath all of those: exported
// and embedded fields, element, key and pointer types, and the exported
// methods' signatures of each such type. Without api the module is judged
// as a program, and an exported field is a subject like any other.
//
// A write is the target of an assignment, op-assignment, inc/dec or range =,
// writing beneath a struct-valued field writing it too; x.f = append(x.f, …)
// on the same selection; a composite-literal element; and a sync/atomic write,
// which is Store, Swap, Add, And or Or called on an atomic-typed field,
// or the package function of one of those names given &x.f, whose result
// is discarded: Swap, Add, And and Or return what the field held, so code using
// that result reads it. Load and CompareAndSwap read. Every other selection
// is a read: &x.f handed on, a method called on the field, an element written
// through it, since x.m[k] = v reads a map that may be shared, and a read
// inside String, Format or an echo function, since what is printed is read.
//
// Equality over a type holding a struct by value reads every field it holds
// by value: == and !=, a switch tag, a map key type, and a type argument
// a comparable constraint admits. Converting the struct to another struct
// type, or a pointer to it to unsafe.Pointer, reads the same fields. Converting
// a value to an interface reads every field reflection reaches beneath it,
// exported or not, through pointers, slices, arrays, maps and channels
// and never a func type, since fmt prints unexported fields
// and reflect.DeepEqual compares them; reflect.TypeFor reaches beneath its
// type argument alike. A value whose interface only ever has its methods called
// or feeds standard-library sinks is the exception: a method called reads what
// its body reads, a sink that holds it, as context.WithValue holds its value,
// reads nothing, and one that compares it, as a key, reads what equality
// does. Evidence is read from the module's own code, live and dead alike,
// a generic body in each instantiation. A dependency's body is not read, so
// what crosses into one is judged at the call, which is why a struct only
// an atomic.Pointer holds is reported.
//
// Evidence counts toward a field only from code written for no wider
// an audience: production code toward a production field, and tests too toward
// a field a test file or a test-facing package declares, a test-facing package
// being written for tests. A production field that only tests or test-facing
// code read is reported as read only by tests.
//
// Which structs a caller hands in is derived by polarity from the api patterns,
// which inert-field requires. The roots are the exported functions
// and variables, and the exported methods of exported non-interface types,
// of the api packages: a function's parameters are authored by the caller
// and its results handed to it, and a variable is both, since the caller
// assigns it and reads it. The walk crosses pointers, slices, arrays, map keys
// and elements, channel elements, type arguments, and the underlying type
// of a named non-struct type, and descends the exported and embedded fields
// of every struct it reaches, staying inside the module. Crossing a func type,
// or an exported method of an interface, flips its parameters' polarity, since
// the other side supplies them, and keeps its results', so what a caller's
// implementation returns is authored. An exported struct type reached authored
// is a subject, and its exported fields are judged, with those a struct
// it embeds by value promotes onto it.
//
// A read is a field selection that is not what an assignment writes,
// in a function production roots reach, outside test files; a function only
// tests keep alive does not act on the field. Reading a whole embedded value
// reads every field it promotes. Converting a value to an interface reads every
// exported field beneath it, since reflection can, so a subject that is printed
// counts as read: the rule claims only what it can show. Handing a value
// to consumer code reads every exported field beneath it too, since that code
// can read any of them: passing it to a method selected, by a call, a method
// value or a method expression, on an interface a consumer can implement,
// or to a call through a func value of a type the walk crosses authored.
// A consumer can implement an interface declared outside the module, one
// exported from a package another module can import that carries no unexported
// method, one written in place with no unexported method, and any interface
// the walk crosses authored, which a consumer satisfies structurally whatever
// its declaration. A generic body is judged in each instantiation. Returning
// a value is no hand-off: it cannot be told apart from an accessor echoing
// a setting back, so a struct the module only stores and returns, as a cache's
// Put and Get do, is reported, and a module of that shape leaves the rule off
// or excludes the type. A read inside a String, GoString or Format method,
// or inside a function echo-functions lists, does not count: echoing a field
// into text or an identity does not act on it. A field whose doc carries
// a Deprecated: paragraph is not judged, since keeping a field that does
// nothing, so callers still compile, is what deprecation means.
//
// inert-field is opt-in because it judges an API's design rather than
// reachability, and a field reserved for a later release is the module's
// decision.
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
// Including tests — the default — additionally reports the engine's test-only
// verdict family: a production declaration that only test code keeps alive
// carries its verdicts prefixed test-only, and one exclusion matching
// that prefix silences the family. Excluding tests behaves exactly as before.
//
// Two settings declare structure the analysis cannot discover. test-facing
// names packages whose intended consumers are tests — a test plugin
// production-shaped enough that the compiler cannot tell — so their own surface
// stops drawing the test-only family while a production declaration only they
// keep alive draws it completely. roots names entry-point files the loader
// cannot reach, conventionally ignore-tagged package main generators run
// with go run; each is loaded as its own single-file program, contributing call
// edges and references without ever being reported on.
//
// Including tests sets an api package's tests aside along with every other
// test, though for a library they are often the only stand-ins for consumers
// the run can see. api-test-consumers counts them as such: in the masked view,
// test evidence aimed at the declared surface counts as production evidence
// would, and test evidence aimed anywhere else stays masked.
//
// enum-names states a convention instead: each enum — a named type its own
// package declares typed constants of, other than a flag set, a quantity
// or a generic type, as internal/enum defines them — publishes a name per
// member through a String method, used or not.
//
// echo-functions serves inert-field, naming functions whose reads echo
// a field rather than act on it, each as import/path.Func,
// import/path.Type.Method or import/path.(*Type).Method with a reason. Given
// while inert-field is off, it fails building the plugin. An entry naming
// no function of the module, or one whose exemption changes no verdict, fails
// the run.
//
// An absent key and an empty list mean different things. Omitting patterns
// analyzes the whole module; writing patterns: [] names no packages at all
// and is rejected rather than quietly read as the default. Omitting api-exempt
// exempts method; writing api-exempt: [] exempts nothing, holding every
// exported declaration of the surface to a reference.
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
