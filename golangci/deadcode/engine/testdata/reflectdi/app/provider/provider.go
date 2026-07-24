// Package provider holds the constraint-satisfying provider and its control.
package provider

// Input is the provider's datum.
type Input struct{ Value string }

// Impl satisfies di.Provider[Input] at the enrollment site; the compiler's
// verification there is the only evidence its methods run.
type Impl struct{}

func (Impl) Configure(name string) {
	if name == "" {
		panic("unnamed provider")
	}
}

func (Impl) ResolveInput(input Input) Input { return input }

// Shadow structurally satisfies the same constraint but is never enrolled,
// boxed, or instantiated against it.
type Shadow struct{}

func (Shadow) Configure(name string) {
	if name == "" {
		panic("unnamed shadow")
	}
}

func (Shadow) ResolveInput(input Input) Input { return input }
