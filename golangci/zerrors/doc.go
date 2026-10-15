// Package zerrors judges the errors a module produces: whether a caller can
// tell them apart, and whether anything decides on their text.
//
// An error that leaves the module's API is something its caller has to act on,
// and a caller can only act on what it can name. A value of an exported error
// type, or a sentinel, is identified with errors.As or errors.Is; an error made
// by errors.New, or by fmt.Errorf wrapping nothing identifiable, can only
// be told apart by reading its text, and text is not a contract. The same holds
// inside the module: a branch on an error's message breaks the day someone
// rewords it.
//
// The analysis lives in the engine subpackage. This package adapts
// it to golangci-lint, as the deadcode linter does, and for the same reason:
// what zerrors judges is a property of the whole module rather than of one
// package.
//
// # Rules
//
// Each diagnostic names the rule that produced it as the first word of its
// message, so a consumer can exclude one rule without silencing the linter:
//
//   - unstructured-return: a return in a judged function that can carry
//     an unstructured error.
//   - text-decision: a branch taken on an error's text.
//   - literal-field: a string or []string field of an error type that only ever
//     holds constants.
//   - rendered-field: a string field of an error type that only ever holds
//     the String of one type its caller could carry instead.
//   - missing-base: an error type that does not carry the configured base.
//
// # Words
//
// A module package is one ./... matches from the module root. Anything declared
// outside one is foreign: the standard library, every dependency, a nested
// module. A trusted package is a foreign one the trusted setting names, whose
// own linting is taken to hold its errors.
//
// An API package is a module package the api patterns match that is not main
// and holds no internal path element. Go already stops another module importing
// an internal package, so no pattern makes one API.
//
// An error type is a defined, non-interface type T where T or *T has Error()
// string. A caller outside the module can name one when it is an exported error
// type of an API package, one an exported alias of an API package names, any
// error type of a trusted package, or, while a base is set, one carrying
// the base, since a caller reads its classification through the base. A generic
// error type is judged at its declaration.
//
// A module interface is closed when the module sees all its implementations:
// it is unexported, declared in a package no other module can import,
// or carries an unexported method. Its implementations are the concrete types
// module code converts to it, foreign ones included, and a blank assertion such
// as var _ I = (*T)(nil) counts as a conversion. Any other interface is open:
// consumers implement it.
//
// # unstructured-return
//
// The judged functions are those a caller outside the module calls, each
// with a result an error is returned in:
//
//   - the exported functions of API packages;
//   - the exported methods in the method set of an exported type of an API
//     package, T or *T, a promoted method being judged at its declaration;
//     a type an exported alias of an API package names is exported there too;
//   - the methods implementing the exported methods of an exported interface
//     of an API package, which consumers call through the interface; a generic
//     interface is implemented through its instances, by the types module code
//     converts to one;
//   - the methods a module type answers a trusted interface with, once module
//     code converts it to one. The framework calls them, and its idiom governs
//     what they return.
//
// Unwrap on an error type is not judged: it hands out the cause for a caller
// to identify. Nor is an implementation of a method the protocols setting
// lists, whose consumer classifies whatever an implementation returns, nor
// a method answering a foreign, untrusted interface module code converts its
// type to. That method answers to the interface's own contract: io.Writer
// expects io.ErrShortWrite.
//
// Each value a judged function returns is traced backward through the module's
// value flow — merges, loops, named results, closures and the calls between
// them — to its origins, which sort into four kinds:
//
//   - Structured: a value of a nameable error type; an exported package-level
//     error variable of an API package while sentinels is on; anything trusted,
//     whether a call, a variable or a call through an interface method;
//     fmt.Errorf with a structured %w operand, or errors.Join with a structured
//     operand; and a parameter of the judged function, which is its caller's
//     own error. Nil and a cause slot's value count for nothing in an operand:
//     one that can be either or structured is structured, and one that can
//     only be either structures nothing.
//   - Unstructured: errors.New; fmt.Errorf or errors.Join that no operand
//     structures and one can make unstructured, and fmt.Errorf wrapping only
//     nil; a value of an error type no caller can name; and anything foreign
//     and untrusted — a call, a call through an interface method such
//     as ctx.Err(), a package-level variable such as io.EOF, a load from
//     a struct's error field, or a receive from a channel such code provides
//     as a field or a call's result.
//   - Followed: a call to a module function, through what it can return under
//     which arguments; a call through a closed interface's method, as every
//     implementation's result together, a foreign implementation's being its
//     own origin; a load from a module struct's error field, as every store
//     into it; and a module variable that is not a counted sentinel, as what
//     initializes and stores into it.
//   - Neutral: a load from a cause slot, an error field of a module error type.
//     Whatever was stored there was judged where it was made, by the function
//     that returned it, so the load is no unstructured origin, and wrapping
//     it with nothing but nil and other cause slots' values is neutral too.
//   - Not judged, and so counted as structured, because the rule reports what
//     it can show and never guesses: calls through function values, and through
//     an open interface, whose implementations live with consumers; a foreign
//     call handed a module function value, such as filepath.WalkDir,
//     and a later call on the receiver of such a call in the same function,
//     such as errgroup.Group.Wait, which return what the callback returned;
//     other channel receives; loads from slices, maps and pointers; type
//     assertions; values of a type parameter; and fmt.Errorf with a format
//     that is not a constant, whose verbs cannot be read.
//
// A finding sits on the return statement and names one origin,
// with the function it sits in when that is another. A return whose every
// unstructured origin comes through a call to another judged function draws
// nothing, since that function's own return carries the finding: a defect
// is reported once, where it is produced.
//
// # text-decision
//
// A text decision is a branch taken on the content of an error's text:
// the result of Error() on any error, fmt.Sprint or fmt.Sprintf of an error,
// or a string field of a module error type, and anything a strings
// transformation, a slice or a local makes of one. What counts as a decision
// is shared with the zenum linter, which judges an enum's rendered names
// the same way: a strings matcher against a non-empty constant, a regexp
// match, or == and != against a non-empty constant, which is also what a case
// of a switch is. Comparing with "" is a presence check, comparing two errors'
// texts, or an error's text with any computed string, decides nothing about
// either, and ordering by text is not a branch on it. A foreign error
// type's fields are its owner's documented API, and are not judged.
//
// A decision on an error's own text should branch on its type or fields.
// A decision on a string field of a module error type reads an enum the field's
// type does not name, so the finding points at the field: give it an enum type
// and branch on the members.
//
// # literal-field
//
// A field of type string or []string, declared on a module error type,
// that some module code stores into and that only ever holds constants should
// have a typed enum instead. The type is compared exactly, so a named
// string type is not judged: naming it is already the typing decision.
//
// A value is constant-derived when it is a constant expression
// or a concatenation of constant-derived operands, a []string literal of them
// or nil, or a local only ever assigned them. It is also constant-derived when
// it is a parameter of an unexported module function only ever called
// statically whose every call passes one, the result of a static call
// to an unexported module function whose every return is one, the String
// of an exported enum of an API package, or a load from a string field
// of a module struct whose every store is one: that is how a carrier struct
// passes its values into the error it raises.
//
// An exported function's callers are not all visible, so its parameters
// are never followed. An exported field of an API package is not followed
// as a source either, since consumers store into it too. An internal type's
// String is a display string, not a member's name, and is not followed.
// Constant-derivation is a greatest fixed point: every field, parameter
// and result starts out constant-derived, and one fed anything that is not
// loses it, as does everything fed by it, until nothing changes. Fields
// of structs other than error types take part, and only error types' fields
// are reported.
//
// A field written in ways no store shows holds what no store says, and is not
// constant-derived: one whose struct is converted from another struct type,
// and an exported one whose struct is handed to foreign code by pointer,
// or converted to an interface with no methods, through which a decoder writes
// it by reflection. Neither foreign code nor reflection can set an unexported
// field. A store in a generic body counts though nothing instantiates it,
// and a value there that depends on a type parameter is not constant-derived.
//
// A field that only ever holds one value is no choice an enum could name:
// it carries no information, and the finding says to remove it instead.
// A field some module code leaves unset — a composite literal leaving it out,
// new, or a variable declared with no value — also holds its zero value,
// and that counts as a second one.
//
// # rendered-field
//
// A field of type string, declared on a module error type, that some module
// code stores into and whose every store is the String of one type T should
// carry T instead, when T is a type a caller could hold: an exported, defined,
// non-interface type of an API package, so the caller can name it; comparable,
// so carrying it gives the caller ==; and not an enum, whose String
// is a member's name and literal-field's to judge. A field holding a rendered
// identifier hands the caller text where the module had a typed value, so
// the caller renders its own key the same way and compares strings: a text
// decision by proxy, which breaks the day String changes format.
//
// Values are followed as literal-field follows them — through locals, through
// the parameters and results of unexported module functions, and through loads
// from fields — and a field written in ways no store shows is not judged.
// A constant, or a String called through an interface, is no rendering, so
// a field mixing sources or holding a caller's own string is not judged.
// A generic body's stores are judged in its instances, where the type whose
// String is called is known: the rule fires when every store renders the same
// instantiated type, and the field then carries that instantiation.
//
// A deliberate rendering, such as one redacting a credential, is a one-off
// suppression with its reason.
//
// # missing-base
//
// While base is set, every module error type has to carry it: a struct base
// embedded by value, directly or through a chain of embeddings by value,
// or an interface base implemented by T or *T. The base itself is not judged,
// nor is an error type in a package the base's own package imports, directly
// or transitively, which could not embed it without an import cycle.
//
// base-packages narrows the rule to the part of the module following the idiom
// the base belongs to, as api says which part is API. Code that stays outside
// the framework on purpose — tooling, an example application, a wrapper
// returning another library's errors by design — is left out by the module's
// structure rather than by suppression, and a pattern that stops matching fails
// the run. A pattern may match main and internal packages, unlike api's:
// an internal package's errors still leave the module, wrapped in API ones.
// It scopes missing-base only. An error type carrying the base counts
// as nameable wherever it is declared.
//
// # How a whole-module result reaches per-package diagnostics
//
// The engine loads the module once, sharing the load with the zenum linter when
// both run under the same build tags: module packages from source,
// and dependencies from export data. The analysis runs once behind
// a [sync.Once], its findings are indexed by file, and each pass reports those
// in the files it holds, so reporting is scoped to the packages golangci-lint
// was asked to lint while the analysis always covers the whole module. A second
// analyzer whose name is unique to the process defeats golangci-lint's
// per-package issue cache, which would otherwise replay a finding an edit
// to some importer had made wrong. Nothing bounds how long the analysis takes,
// for the reasons the deadcode linter gives.
//
// The errors a linter's run returns reach golangci-lint through
// analysis.Analyzer.Run, a function value, which is not followed. A module
// that runs this linter on itself is not judged on them.
//
// # Settings
//
// The plugin cannot read golangci-lint's run configuration, so build tags
// are a setting of this linter's. An absent key and an empty list differ:
// omitting api judges every package as API, and writing api: [] names no
// package and is rejected; the same holds of base-packages, which is rejected
// too when base is not set, since it would scope a rule that is off.
// A protocols entry carries a reason, which has to be written out: such
// an entry is a property of the interface rather than a one-off suppression.
//
// Every rule is standard, and the default, enable and disable keys choose which
// run, as every linter of this module chooses. api serves unstructured-return,
// literal-field and rendered-field; trusted, protocols and sentinels serve
// unstructured-return; base serves unstructured-return and missing-base;
// and base-packages serves missing-base. Giving one while every rule it serves
// is off configures nothing, and fails building the plugin.
//
// Settings no module could satisfy fail building the plugin. Whatever needs
// the module to check fails the run instead: an api pattern matching no API
// package of the module, a trusted entry matching no package the module
// imports, directly or transitively, a protocols entry naming no interface
// method or exempting nothing that is not exempt already, a base that does not
// resolve or names neither a struct nor an interface, and a base-packages
// pattern matching no package of the module. Configuration matching nothing
// does not mean what it says.
//
// # Suppression
//
// golangci-lint applies the nolint directive centrally, matched on the linter
// name, so a one-off suppression goes through it or through
// a linters.exclusions rule matching the rule token a message opens with.
//
// [sync.Once]: https://pkg.go.dev/sync#Once
package zerrors
