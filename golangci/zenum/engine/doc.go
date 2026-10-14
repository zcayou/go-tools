// Package engine judges a module's enums, and every use of them in module code.
//
// It is the analysis itself, with no opinion about how findings are presented.
// The parent package adapts the whole-module result to go/analysis's
// per-package contract and states the rules and their reasoning. They
// are separate packages because each names the same idea differently: [Config]
// is what the analysis needs, and the parent's Settings is a decoded YAML block
// whose reasons the analysis has no use for.
//
// [Analyze] takes a module another caller loaded, rather than loading one
// itself, so that zerrors and this engine read one load when both run: module
// packages from source with type information, dependencies from export data,
// and no tests. Nothing here needs a dependency's bodies. A dependency's
// enum is no rule's subject, and what the module does with one
// is in the module's own source.
//
// # The enums
//
// An enum is what internal/enum says it is, narrowed to types a module package
// declares with an integer or string underlying type: flag sets, quantities
// and generic types are none, and count sentinels are no members. The word
// means the same thing in every linter of this module. Its tokens are read
// once, from the declaring package's syntax, and the enums are kept
// in import-path-then-name order, which is the order restated places its
// finding by. An open-enums entry removes its type before anything else looks,
// so every rule passes it over alike.
//
// # Two readings of the module
//
// Each rule reads whichever form keeps what it turns on.
//
// The SSA program decides visibility and the decision form of token-decision.
// Conversion to an interface is explicit there: a MakeInterface instruction,
// whether the source wrote an assignment, a fmt argument, an any parameter
// or a variadic operand. Generic bodies are read once per instantiation
// the module builds, so an enum handed to a generic function that prints its
// type parameter is converted where it is instantiated, which the generic body
// as written cannot show. A rendered token is a value with a source —
// the static call to an enum's String, promoted wrappers included,
// the conversion of a string enum to a string type, or a call to fmt.Sprint
// with an enum operand — and internal/decision follows it through strings
// transformations and locals to the decisions made on it. An instantiated body
// is also where a conversion to a type parameter becomes a conversion into
// an enum: its conversion instruction carries the position of the conversion
// the generic body writes, which the syntax reading records.
//
// The syntax decides the rest. A comparison of an enum value with a constant
// turns on how the constant is spelled — a member named, or zero written
// as a number — and SSA keeps a member's name and the number it stands
// for as one constant. A conversion of a constant folds away in SSA, so
// Kind(99) leaves nothing behind to report, and neither does the untyped 99
// in Take(99), which the type checker records with the enum's type. A format
// string's verbs pair with the operands written at the call. Each is read off
// the syntax tree with its type information, in every non-test file of every
// module package, one declaration at a time, so that a method of an enum's own
// type is known as one.
//
// The functions read are those the module's syntax declares, its package
// initializers, which hold package-level initialization, the methods of every
// module type the program converts to an interface, and what those reach
// that is still module code: closures, and the instantiations the program
// built. A method reached only through an interface is called by no
// instruction, and an instantiated method of a generic type exists only once
// the program is asked for it, so the program's runtime types are walked
// for their method sets. [ssa.Program.MethodValue] builds such a method
// on demand under the program's own locks, which is safe while other linters
// read the same load. A method with type parameters of its own has no method
// value, and is read through the instantiations its callers make.
//
// # Visibility
//
// Exposure is computed as a closure over exported declarations. Each exported
// variable, constant, type and function exposes the named types its type
// carries; an exposed type exposes, in turn, what the signatures of its
// exported methods carry, and an exposed interface what its methods' do. So
// an unexported type an exported function returns publishes its methods
// as an exported one does, while an exported method of a type nothing exposes
// publishes nothing.
//
// Carriage follows pointers, slices, arrays, maps and channels, a named type
// into its underlying type, and a struct into its exported and embedded fields.
// It stops at an unexported field, at a function type and at an interface,
// none of which hands a reader the value itself.
//
// A conversion to an interface inside a method of an enum's own type does not
// make that enum visible, and neither does one whose interface is handed only
// to the opaque sinks a table in visibility.go lists: standard library
// functions that key, store, sort or describe the type of a value and never
// render it.
//
// # Format strings
//
// A fmt-style call is a call to a function or method whose name ends in f
// and whose parameters end in a format string followed by ...any — fmt's own,
// log's, and wrappers written in their image. Its format is read only when
// it is a constant, and its operands only when they are written at the call
// rather than spread from a slice. Verbs pair with operands as fmt's own
// parser pairs them: in order, with a * width or precision consuming an operand
// of its own and an explicit [n] index moving the count. A verb after
// a malformed or out-of-range index, or past the last operand, formats none,
// as fmt prints it as a bad or missing operand.
//
// # Exceptions
//
// An exception that suppresses nothing is reported as an [*ExceptionError]
// once the whole module has been judged, so that a run reports every stale
// entry together, joined, rather than one per attempt. A failed run reports
// no findings.
//
// # Findings
//
// A finding's position is resolved without //line adjustment, so it names
// the file on disk, which is what a go/analysis pass is looked up by.
// A generic body read once as written and once per instantiation finds
// the same thing more than once, so a finding is recorded once however many
// readings produced it.
package engine
