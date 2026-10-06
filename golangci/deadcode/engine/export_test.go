package engine

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// The specs are black-box files like every other test in the module,
// and the unit-level components they exercise are deliberately engine-internal.
// This bridge re-exports those internals for them and carries nothing else.
// The constructors are exposed under the full view — masking is exercised
// through Analyze over whole-module fixtures, where the loaded program actually
// carries test variants.

// Program exposes program, the completed SSA program the engine reads.
type Program = program

// BuildProgram exposes buildProgram.
var BuildProgram = buildProgram

// Functions exposes the completed program's function set.
func (p *program) Functions() map[*ssa.Function]bool {
	return p.funcs
}

// NewInstantiations exposes newInstantiations.
var NewInstantiations = func(loaded []*packages.Package) *instantiations {
	return newInstantiations(loaded, view{})
}

// Substitute exposes substitute.
var Substitute = substitute

// MaxResolvedVectors exposes the cap vectors applies.
const MaxResolvedVectors = maxResolvedVectors

// Vectors exposes instantiations.vectors.
func (s *instantiations) Vectors(obj types.Object) [][]types.Type {
	return s.vectors(obj)
}

// NewEvidence exposes newEvidence, with no declared surface to seal anything.
var NewEvidence = func(prog *program, inst *instantiations) *evidence {
	return newEvidence(prog, inst, &sealedSurface{}, view{})
}

// Materialized exposes evidence.materialized.
func (e *evidence) Materialized(method *types.Func) bool {
	return e.materialized(method)
}

// Conversions exposes evidence.conversions.
func (e *evidence) Conversions(yield func(operand, iface types.Type) bool) {
	e.conversions(yield)
}

// Granted exposes the closure membership grant behind materialized, so the skip
// semantics are assertable on types no method could name.
func (e *evidence) Granted(t types.Type) bool {
	return e.evident(t)
}

// Closure exposes every type the closure visited with its grant, so the whole
// closure is comparable against RTA's runtime types.
func (e *evidence) Closure(yield func(t types.Type, granted bool) bool) {
	done := false
	e.granted.Iterate(func(t types.Type, value any) {
		if done {
			return
		}
		granted, _ := value.(bool)
		done = !yield(t, granted)
	})
}

// NewFileFacts exposes newFileFacts.
var NewFileFacts = newFileFacts

// NewMethodReferenceScan exposes newMethodReferenceScan.
var NewMethodReferenceScan = func(
	pkgs []*packages.Package,
	facts map[*packages.Package]fileFacts,
	ev *evidence,
	inst *instantiations,
) *methodReferenceScan {
	return newMethodReferenceScan(pkgs, facts, ev, inst, view{})
}

// NewInterfaceFlows exposes newInterfaceFlows.
var NewInterfaceFlows = func(prog *program, inst *instantiations, refs *methodReferenceScan) *interfaceFlows {
	return newInterfaceFlows(prog, inst, refs, view{})
}

// Used exposes interfaceFlows.used.
func (f *interfaceFlows) Used(key string) bool {
	return f.used(key)
}

// MethodKey exposes the declaration key the scans and flows are indexed by.
func MethodKey(fset *token.FileSet, obj types.Object) string {
	return declKey(position(fset, obj.Pos()))
}
