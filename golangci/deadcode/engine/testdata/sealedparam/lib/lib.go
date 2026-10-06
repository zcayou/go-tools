// Package lib seals a generic contract whose method mentions the parameter,
// the shape of a typed declaration compiled into a typed result.
package lib

// Compiled is the typed result a declaration compiles to.
type Compiled[I any] struct{ name string }

// SchemaFor is sealed, and its method returns a type over its parameter, so
// a receiver as written matches no instantiation of it.
type SchemaFor[I any] interface {
	compile() (Compiled[I], error)
}

// Compile is the only caller of compile.
func Compile[I any](declaration SchemaFor[I]) (Compiled[I], error) { return declaration.compile() }

// Tagged is the implementation production reaches.
type Tagged[I any] struct{}

func (Tagged[I]) compile() (Compiled[I], error) { return Compiled[I]{name: taggedHelper()}, nil }

// Default is how production arrives at Tagged.
func Default() (Compiled[string], error) { return Compile[string](Tagged[string]{}) }

// Explicit is the public builder's implementation, which only the test hands
// to Compile, standing in for a consumer.
type Explicit[I any] struct{}

func NewExplicit[I any]() Explicit[I] { return Explicit[I]{} }

func (Explicit[I]) compile() (Compiled[I], error) { return Compiled[I]{name: explicitHelper()}, nil }

// Unbuilt mentions the parameter too, and nothing anywhere instantiates it, so
// no instantiation a consumer is known to hold implements the contract.
type Unbuilt[I any] struct{}

func (Unbuilt[I]) compile() (Compiled[I], error) { return Compiled[I]{name: unbuiltHelper()}, nil }

// Labeled is sealed the same way, but its method leaves the parameter out,
// so a receiver as written satisfies every instantiation of it.
type Labeled[I any] interface{ label() string }

type Label[I any] struct{}

func (Label[I]) label() string { return labelHelper() }

// Describe is the only caller of label.
func Describe[I any](labeled Labeled[I]) string { return labeled.label() }

func taggedHelper() string { return "tagged" }

func explicitHelper() string { return "explicit" }

func unbuiltHelper() string { return "unbuilt" }

func labelHelper() string { return "label" }
