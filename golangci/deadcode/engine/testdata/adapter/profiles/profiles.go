// Package profiles declares interfaces whose methods consumers select only
// through identically re-declared adapters in main.
package profiles

// Profiles feeds a plain re-declared adapter by value; the conversion between
// the identical method sets is the only evidence this declaration
// is load-bearing.
type Profiles interface {
	List() []string
}

type prof struct{}

func (prof) List() []string { return []string{"profile"} }

// New returns the in-module implementation behind Profiles.
func New() Profiles { return prof{} }

// Registry is supplied as a type argument satisfying an identical adapter
// constraint.
type Registry interface {
	Names() []string
}

type reg struct{}

func (reg) Names() []string { return []string{"registry"} }

// NewRegistry returns the in-module implementation behind Registry.
func NewRegistry() Registry { return reg{} }

// Evaluator is bridged to main's identical copy by closures converting
// in opposite directions; only the copy is ever selected.
type Evaluator interface {
	Snapshot() map[string]string
}

type eval struct{}

func (eval) Snapshot() map[string]string { return map[string]string{"key": "value"} }

// NewEvaluator returns the in-module implementation behind Evaluator.
func NewEvaluator() Evaluator { return eval{} }

// Ignored is the control: identical to Profiles, and flowing nowhere.
type Ignored interface {
	List() []string
}

// Watched is the assertion control: it reaches an adapter only through
// a type assertion, which requires no static satisfaction.
type Watched interface {
	List() []string
}
