// Package zenum exposes the enum engine as a golangci-lint linter.
//
// An enum is a defined, non-generic type with an integer or string underlying
// type whose own package declares typed constants of it in non-test files.
// Those constants are its members, judged by value, so a constant re-exporting
// a member under another name is still that member. The linter holds such
// a type to one principle: it is ordinal, names itself, and is the only way
// to make its values. Its String gives every member one distinct token, its
// zero value is not a member, no second enum restates its tokens, and no code
// outside its package converts an arbitrary number into it.
//
// Only enums the module declares are judged. A dependency's enum is its owner's
// business, and is no rule's subject here.
//
// How the constants are declared can say a type is no closed set at all. Each
// of these reads the declaration alone, never how the constants are used:
//
//   - A flag set is no enum. Its values combine, so a value no constant
//     names is ordinary. A type is one when any of its constants is declared
//     by a << shift, written or repeated from the line above, as in 1 << iota.
//   - A quantity is no enum. Its values are amounts and its constants
//     units. A type is one when any of its constants is declared by multiplying
//     another of them, as in MB = 1024 * KB. Adding to one, as in B = A + 1,
//     is how an enumeration counts, and keeps the type an enum.
//   - A count sentinel is no member. It marks where an enumeration ends rather
//     than naming a value of it. A constant is one when it is unexported,
//     it closes an iota run — it is the last line of its const block
//     and repeats the run's iota expression — and its own package uses
//     it as an array length or as one side of an ordering comparison. Its type
//     stays an enum; the sentinel needs no token, and no rule counts its value
//     as a member's.
//
// # Rules
//
// Each diagnostic names the rule that produced it as the first word of its
// message, so a consumer can exclude one rule without silencing the linter:
//
//   - string-enum: an enum with a string underlying type. Its tokens are its
//     identities, so renaming one changes what compares equal, and converting
//     any string, such as Kind(input), makes a value no declaration names.
//   - unnamed: a visible ordinal enum with no String() string on a value
//     receiver. A pointer receiver names no value, since a value is what fmt
//     and every reader holds.
//   - unreadable-name: a visible ordinal enum whose String is in no readable
//     form. What it publishes cannot be checked, so the enum's names go
//     unverified.
//   - ambiguous-name: a readable String that gives some member value no token,
//     an empty token, or a token another member value also gets. Members
//     sharing one value share one token, and are not ambiguous for it.
//   - zero-member: a visible enum with a member whose value is zero. Zero
//     is what a field that was never set holds, so a member at zero cannot
//     be told apart from no choice at all.
//   - restated: two enums, each with at least two tokens, whose token
//     sets are equal or one of which contains the other. That is one concept
//     spelled twice, and the spellings drift apart the first time one
//     is renamed. The finding sits on the later of the two
//     in import-path-then-name order, and names the earlier.
//   - token-decision: a text decision on a rendered token — on v.String(),
//     string(v) of a string enum, or fmt.Sprint(v) of an enum value. Also
//     an enum value compared or switched against a constant that names no
//     member: one no member holds, as k == 7 where k == Running was meant, and,
//     outside the declaring package, one spelling a member's value, as k == 2
//     where KindB holds 2, which keeps comparing with 2 after the constants
//     are renumbered.
//   - forged-conversion: a conversion into an enum, outside its declaring
//     package, of anything but a constant holding a member's value or zero.
//     Kind(n) makes a value no declaration names. Parsing a token or decoding
//     a number into a member belongs to the declaring package. The conversion
//     need not be written: an untyped constant the source gives the enum's
//     type, as in Take(42), var k Kind = 7 or KindB + 5, is one, judged where
//     the constant is written or, for a constant declared with the enum's type,
//     where it is declared. A constant compared with, cased on or looked up
//     by makes no value, and is token-decision's. So is a conversion to a type
//     parameter that an instantiation with the enum turns into one, as in T(n)
//     in Convert[Kind]; a constant operand folds away there, so only a computed
//     one is seen.
//   - numeric-verb: an ordinal enum value a printer formats under %d, %x, %X,
//     %o, %b, %c or %U. Those print the ordinal, or the bytes of the name
//     in hex, and never the member's name. A method of the enum's own type
//     is exempt for its own values: its String printing a value no member holds
//     as "Kind(%d)" is the one place the ordinal should print, and %v there
//     would call String again.
//   - string-bypass: an ordinal enum value a printer formats for its name,
//     under %v, %s or %q, where fmt calls no String: beneath an unexported
//     field, whose methods fmt cannot call, or as the elements of a slice
//     or array whose kind is uint8 under %s or %q, which fmt prints as raw
//     bytes. %#v asks for Go syntax and is not judged.
//   - laundered-conversion: an ordinal enum value converted, outside its
//     declaring package, to a non-interface type other than its own and other
//     than another enum's, which is forged-conversion's. int(kind) keeps
//     the member's ordinal and drops its type: the number travels into a row,
//     a label or a key nothing ties back to the declaration assigning it,
//     and changes when a member is inserted. The declaring package owns its
//     encoding in both directions. A conversion handed directly
//     to a sync/atomic integer's Store, Swap, CompareAndSwap or Add keeps
//     the ordinal without reading it, and comes back a member only through
//     a conversion into the enum, which forged-conversion judges. A conversion
//     to or from a type parameter is judged in each instantiation, and reported
//     where the generic body writes it.
//   - ordered-comparison: an ordinal enum ordered outside its declaring
//     package, by <, <=, > or >= or by the min or max builtin, unless
//     ordered-enums lists it. Most enums are unordered sets that happen
//     to be numbered: State > Running assumes declaration order is lifecycle
//     order, and silently admits whatever member is inserted between them. Only
//     an ordering written on the enum's own type is judged. An ordering
//     of a type parameter, and so a sort instantiated with the enum, decides no
//     membership.
//
// # Formatting
//
// numeric-verb and string-bypass judge every printer's operands. A printer
// is one of the standard-library functions and methods vet's printf analyzer
// knows by name; a module function or method that forwards its final ...any
// parameter, spread and unaltered, to a printer, found to a fixed point as vet
// finds wrappers, printf kind when it also forwards the string before
// it as the format; a method of a module interface every module type converted
// to it implements with a printer of one kind; or a foreign function or method
// whose body the load does not hold, when its name ends in f and its parameters
// end in a format string and ...any. A print-kind printer formats every operand
// as %v.
//
// An operand is judged for everything fmt descends into under its verb: struct
// fields, exported or not, slice and array elements, map keys and values,
// and a pointer's target at the top level. fmt stops at a value it hands
// to the value's own method — a Formatter under every verb, an error
// or Stringer under %v, %s, %q, %x and %X — but only where it can call one,
// which is nowhere beneath an unexported field. An operand whose type holds
// a type parameter is judged in each instantiation the module makes,
// and reported where the generic body passes it.
//
// # Tokens
//
// An enum's tokens are what its members render as. A string enum's are its
// members' values. An ordinal enum's are what its String returns for each
// member's value, read rather than run: the reader follows assignments, local
// declarations, increments, if and switch statements with fallthrough,
// conversions, arithmetic wrapped to the type's width, and len, and reads names
// out of package-level array, slice and map literals. That covers a switch
// on the receiver, an if chain, a table indexed by the receiver at an optional
// offset behind a bounds check, and the three layouts stringer generates: one
// run, several runs, and the map.
//
// A member gets a token only where the body reaches a constant string
// for that member's value specifically: one read out of a table by an index
// computed from the receiver, or one reached inside a branch entered
// by an equality on the receiver that holds for the value — a switch case,
// a case of a tagless switch, or an if condition such as k == A. A string
// returned before any condition, switch or table lookup is what every value
// renders as, and is each member's token. Anything else the body reaches is its
// fallback for a value no member holds, and gives the member no token, which
// is an ambiguous-name: a switch's default arm, the return after an if chain
// or a switch none of whose cases matched, the return after a lookup that found
// nothing, the empty string a map holds for a key it lacks, and a panic.
//
// A String is unreadable when, for some member, the body leaves what the reader
// follows — a loop, a condition on something no form names, a member's own
// branch returning a name computed by a call — or when it gives no member
// a token at all. Every rule comparing tokens skips it.
//
// # Visibility
//
// unnamed, unreadable-name and zero-member judge only a visible enum: one whose
// values a reader outside its package can meet. An enum that is never visible
// is the package's private state, and how it names itself is the package's own
// affair. An enum is visible when:
//
//   - it is exported;
//   - it is an error type;
//   - an exported declaration exposes it: as the type of an exported variable
//     or constant, the type of an exported field, or a parameter or result
//     of an exported function or method. A method counts once its type
//     is exposed, so an unexported type an exported function returns publishes
//     what its exported methods return, and an exported interface publishes
//     what its methods do;
//   - module code converts a value carrying it to an interface. Passing a value
//     to fmt, or to an any parameter, is such a conversion. Two are not: one
//     inside a method of the enum's own type, which is the enum's own business,
//     and one handed only to a standard-library sink that never renders what
//     it is given — context.WithValue, context.Context's Value, sync.Map's
//     methods, atomic.Value's Store, Swap and CompareAndSwap, sort.Slice,
//     sort.SliceStable, sort.SliceIsSorted, and reflect.TypeOf.
//
// A value carries an enum through pointers, slices, arrays, maps and channels,
// and through the exported and embedded fields of the structs it holds.
// An unexported field is the package's own state, and a function type holds no
// value until it is called.
//
// # Text decisions
//
// A text decision is a branch on the content of a string: a strings matcher
// such as Contains, HasPrefix or EqualFold between the string and a non-empty
// constant, a regexp match, == or != with a non-empty constant, or a case
// of a switch on the string. A strings transformation of a rendered token, such
// as ToLower or a slice of it, is still that token. Comparing with ""
// is a presence check, and comparing a rendering with another string, by ==
// or by a matcher such as EqualFold, decides nothing about its content:
// that is how the declaring package parses a token. Ordering by text is not
// a branch on content. None of them is a decision.
//
// The comparison form of token-decision reads how the constant is spelled.
// A member named, under its own name or a re-exported one, is what such
// a comparison should say. Zero spelled any other way is a presence check while
// zero is no member, and is reported once zero is one, because it is then
// that member spelled as a number. Any other member's value spelled as a number
// is reported outside the declaring package, which owns the numbering
// and may spell it, as stringer's generated String does.
//
// # Settings
//
// Every entry is a property of a type, named as import/path.Name and applied
// wherever the type is used, and carries a reason:
//
//	settings:
//	  open-enums:
//	    - {type: example.com/wire.ID, reason: IDs come from the wire}
//	  zero-defaults:
//	    - {type: example.com/policy.Mode, reason: zero is the documented default}
//	  related-enums:
//	    - {first: example.com/a.Status, second: example.com/b.State, reason: ...}
//	  ordered-enums:
//	    - {type: example.com/log.Level, reason: severity rises with the ordinal}
//	  build-tags: [integration]
//
// open-enums takes a type whose values come from outside the module, declaring
// a few well-known constants, out of every rule. zero-defaults exempts an enum
// whose zero member is its documented default from zero-member. related-enums
// exempts a pair, in either order, whose tokens overlap because they classify
// different things from restated. ordered-enums exempts an enum whose ordinals
// are an order, as a severity's are, from ordered-comparison. build-tags
// repeats golangci-lint's run.build-tags, which a plugin cannot read.
//
// Every rule is standard, and the default, enable and disable keys choose which
// run, as every linter of this module chooses. zero-defaults serves
// zero-member, related-enums serves restated, and ordered-enums serves
// ordered-comparison; giving one while the rule it serves is off configures
// nothing, and fails [New].
//
// An entry with an empty reason, or naming a type other than
// as import/path.Name, fails [New]. An entry that suppresses nothing fails
// the run, because configuration matching nothing does not mean what it says:
// a type that is no enum of the module, a zero-defaults type with no member
// at zero, a related pair that does not restate, and an ordered-enums type
// nothing outside its package orders. Whether a type is an enum takes
// the module to answer, which golangci-lint constructs a plugin without, so
// those wait for the analysis.
//
// A one-off exception is not a setting. golangci-lint applies the nolint
// directive centrally, matched on the linter name, so a "nolint:zenum" comment
// suppresses one finding, and a linters.exclusions rule matching the rule
// token suppresses a rule. The directive is spelled out rather than written
// here: golangci-lint reads one out of any comment line, trimming leading
// slashes and spaces first, so writing it would suppress findings on whatever
// followed.
//
// # How a whole-module result reaches per-package diagnostics
//
// Whether an enum is visible, and whether two restate each other, turn
// on every package of the module at once, and conversions and comparisons live
// in importers, where go/analysis facts cannot carry them back. So the engine
// runs once per invocation over a typed load of the whole module, and each pass
// reports the findings in the files it holds. The load is the one zerrors
// reads, made once per process for a module root and build tag set, so enabling
// both linters costs one load. The plugin asks golangci-lint for syntax only,
// and never forces type checking on a consumer.
//
// golangci-lint caches issues per package, keyed on the package and its
// imports, while a result here changes with the package's importers. So
// the linter returns a second analyzer whose name is unique to the process,
// which forces that cache to miss and makes every run pay the full analysis.
// Nothing bounds how long that takes: go/analysis carries no context,
// and golangci-lint's run.timeout is a deadline its analysis runner discards.
package zenum
