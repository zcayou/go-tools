// Package engine judges the errors a module produces.
//
// It is the analysis itself, with no opinion about how findings are presented.
// The parent package states the rules and their reasoning, and adapts
// the whole-module result to go/analysis's per-package contract; this package
// explains how the rules are decided. [Analyze] takes a module another caller
// loaded, so that the linters sharing a load share it.
//
// # Why this is not a go/analysis analyzer
//
// Every rule reads something that lives in a package's importers: which
// concrete types reach a module interface, every store into a field, every call
// passing a parameter. Analysis facts flow only from a package to its
// importers, and would force golangci-lint to type check every dependency
// besides, so the engine reads the whole module at once, in SSA form, built
// with every generic the module instantiates instantiated.
//
// # The index
//
// One walk over the module's code gathers what every rule reads. Its functions
// are the module's declared functions and methods, generic ones as their own
// bodies, their closures, the instances the module builds of its generics,
// the wrappers the builder makes for them, and the methods reached only through
// an interface. A method with type parameters of its own has no body until
// instantiated, and is reached through its instances alone. Over those
// functions the walk records each store into a field or a package variable,
// each conversion of a concrete type to an interface, each static call of each
// function, each call through each interface method, each function put to any
// other use, and each closure an anonymous function is made into. A blank
// variable such as var _ io.Writer = (*T)(nil) has no effect at run time, so
// the builder emits no code for it, and the conversion it states is read from
// the syntax instead.
//
// A generic declaration's own body is written over type parameters and never
// runs; its instances do. Conversions are taken from instances only. Stores
// and calls are taken from both: unstructured-return judges no value of a type
// parameter, and literal-field holds a value depending on one not
// constant-derived, so a body nothing instantiates still counts, and its
// instances add nothing it does not.
//
// # Tracing an error
//
// unstructured-return traces each result a judged function returns backward
// through the SSA value flow. A merge is the union of its edges, and a loop
// adds nothing its head does not already reach. A load from a local whose
// address the function took is what the stores reaching that load along every
// path put there. A closure capturing the local may store into it too, and what
// it stores is added when the closure can have run since: when a call,
// the deferred calls, or a go statement lies between the load and a store
// reaching it, or the closure runs in a goroutine of its own. A range-over-func
// loop's body is such a closure, storing the results a return inside the loop
// gives, and a return after the loop does not read them. A local whose address
// goes anywhere else may be written unseen, and is not judged.
//
// What a value can be records, besides its unstructured origins, whether it can
// be nil, whether it can be what a cause slot holds, and which parameters,
// or for a closure which captured variables, it can be. A call to a function
// with a body is read through the function's summary of that for each result.
// The caller resolves the parameters against its own arguments, and a closure's
// captured variables against the bindings of the closure it calls: a captured
// local is whatever is stored into it, judged in the frame of the function
// declaring it. That is what makes a helper passing its argument through
// structured when the argument is, and a closure returning its parent's
// parameter as structured as the parent's caller makes it.
//
// fmt.Errorf with %w and errors.Join make a value structured when one operand
// can be a structured error and cannot be an unstructured one. Nil and a cause
// slot's value count for nothing: an operand that can be either or structured
// is structured, as an error is behind a check that it is not nil, and one
// that can only be either structures nothing. A wrapping whose operands can
// only be nil or a cause slot's value is unstructured by none of them:
// it is neutral, or nil when errors.Join drops every operand, and only
// fmt.Errorf wrapping nothing but nil makes an unstructured error of its own.
// An operand structured only when some parameters are makes the wrapping
// conditional on them, and the condition travels with the summary to be settled
// against each call's arguments, so a helper joining its parameter with a raw
// error is structured exactly when its caller hands it a structured one.
// A judged function's own parameters are its caller's errors, so a wrapping
// conditional on them is structured there.
//
// A summary marks what a judged callee can return as that callee's, which
// is how a defect is reported once: a return whose every unstructured origin
// is a judged callee's draws nothing. Whether the callee carries it is read
// from the callee's own body. A generic judged function's body does not judge
// a value of its type parameter, so what an instance's type argument makes
// unstructured stays with the caller instantiating it.
//
// Summaries are computed on demand, and a call met while its own callee
// is still being summarized reads what is known of it so far. So the engine
// summarizes every judged function in passes, each recomputing every summary
// from what the last one found, until a pass changes nothing. Whether
// a wrapping is structured does not only grow with what its operands can be,
// since an operand that could only be nil comes to structure it once
// a structured origin reaches it, so the passes are bounded too. The witness
// a summary names is the earliest by file name and offset, so every run names
// the same one.
//
// A value reached outside any call — what a field or a package variable
// is stored with, or what flows into a function from all its callers —
// is traced with no call to resolve parameters against. A parameter there
// stands for every argument the function is called with, statically or through
// a closed module interface it implements. A judged function's parameter is its
// caller's own error, and one whose callers are not all known is not judged:
// a function put to other uses than calls, or a method an interface outside
// the module's sight can dispatch to.
//
// A load from an error field is every store to that field anywhere in module
// code, unless the field is a cause slot: an error field of a module error
// type. A cause slot is not followed. What was stored there was judged where
// it was made, by the function that returned it, and the stores into every
// value of the type together say nothing of the value a load reads, so
// the load's value is neutral: no origin of a finding, and counted for nothing
// as a wrapping's operand.
//
// A closed interface's call is the union of what each implementation's method
// returns, read through its summary with the call's arguments, the receiver
// being the interface value. The implementations are the concrete types module
// code converts to the interface, so a type that reaches it only through
// a conversion between interfaces is missed, and the rule stays quiet about it.
//
// A foreign call handed a module function value is not judged, nor is a later
// call on the variable it was made on. That variable is the value at the root
// of the loads and field selections reaching the receiver, along
// with the fields selected, so two calls on one struct's field are made
// on the same one.
//
// # Constant-derivation
//
// literal-field decides each field, parameter and result as a greatest fixed
// point. Each is built once into the nodes its values depend on, or marked
// as fed something that is not constant-derived whatever the nodes hold. Every
// node starts out constant-derived; the marked ones lose it, and so, through
// the graph of dependents, does every node depending on one that lost it.
// What is left once nothing more is lost only ever holds constants. Which
// ones is read afterwards over the same graph, only to tell a field holding
// one value, which carries no information, from one holding several.
//
// A field can be written in ways no store into it shows, and is then not
// constant-derived: when its address is put to a use other than a load
// or a store, when its struct is converted from another struct type, or when
// foreign code may write it. A pointer to its struct handed to foreign code,
// or converted to an interface with no methods, may be written by code
// that does not know the type, by reflection as a decoder does; such code
// cannot set an unexported field, so only the exported fields are written,
// and those of the structs held by value in exported or embedded fields.
// A local whose address escapes is not constant-derived either. A method
// that an interface call can reach has callers that are not all static, so its
// parameters are not followed. A generic body's own parameters and results
// are not followed: they depend on its type arguments.
//
// # Configuration the module cannot satisfy
//
// [Analyze] fails with a [*ConfigError] when an entry means nothing
// in the module: an api pattern matching no API package of the module,
// a trusted entry matching no foreign package the module imports, directly
// or transitively, a protocols entry naming no interface method or taking no
// method out of judgment that nothing else takes out, or a base that does not
// resolve or is neither a struct nor an interface. Such configuration does not
// mean what it says, and passing it silently would leave a rule weaker than
// the configuration claims. A protocols entry or a base may name a declaration
// in any package the module imports, directly or transitively: the shared load
// holds each one complete.
package engine
