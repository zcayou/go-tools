package engine

import (
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// interfaceFlows propagates interface-method use across interface-to- interface
// flows. When interface A flows into interface B — an SSA conversion,
// or A supplied as a type argument satisfying an interface constraint B — a use
// of B's method is a use of A's declaration of that method: the declaration
// is required for the flow to compile, and dynamic dispatch through B enters
// whatever implementation sits behind A. A TypeAssert between interfaces
// is deliberately not an edge: an assertion requires no static satisfaction, so
// the source interface's declaration is not load-bearing there.
//
// Identical method sets convert as ChangeType — which is what an identically
// re-declared adapter produces — and narrowing converts as ChangeInterface, so
// both instructions are edges. Conversions whose operand or result is a type
// parameter are excluded: the instantiated bodies carry the same flow
// with concrete endpoints.
type interfaceFlows struct {
	marked map[string]bool
}

// flowEdge is one interface-to-interface flow, precomputed to what propagation
// needs: the source's method set to resolve into, and the target interface's
// methods with concrete signatures.
type flowEdge struct {
	source  *types.MethodSet
	methods []flowMethod
}

// flowMethod is one target-interface method. inScan says whether
// the interface-method scan holds its declaration; a method outside the scan —
// a dependency's, anonymous, generated-file or test-declared interface — counts
// as used unconditionally, for the reason confersUse treats those interfaces
// as unconditional: their call sites are invisible.
type flowMethod struct {
	decl   *types.Func
	sig    *types.Signature
	key    string
	inScan bool
}

// newInterfaceFlows collects the edges and propagates use to a fixpoint.
// The roots are the interface methods whose direct selections pass the existing
// span filtering.
func newInterfaceFlows(prog *ssa.Program, inst *instantiations, refs *methodReferenceScan, v view) *interfaceFlows {
	flows := &interfaceFlows{marked: map[string]bool{}}
	for key := range refs.interfaceMethods {
		if refs.interfaceMethodUsed(key) {
			flows.marked[key] = true
		}
	}
	flows.propagate(prog.Fset, refs, flowEdges(prog, inst, refs, v))
	return flows
}

// used reports whether the interface-method declaration at key is used:
// selected directly, or reached by propagation.
func (f *interfaceFlows) used(key string) bool {
	return f.marked[key]
}

// propagate marks, for every edge whose target method is used, the source's
// declaration of that method, until nothing changes. A resolution landing
// outside the scan is a no-op — it carries no verdict.
func (f *interfaceFlows) propagate(fset *token.FileSet, refs *methodReferenceScan, edges []flowEdge) {
	for changed := true; changed; {
		changed = false
		for _, edge := range edges {
			for _, method := range edge.methods {
				if method.inScan && !f.marked[method.key] {
					continue
				}
				implementation := resolveMethodAs(edge.source, method.decl, method.sig)
				if implementation == nil {
					continue
				}
				key := declKey(position(fset, implementation.Pos()))
				if _, inScan := refs.interfaceMethods[key]; inScan && !f.marked[key] {
					f.marked[key] = true
					changed = true
				}
			}
		}
	}
}

// flowEdges collects every interface-to-interface flow: SSA conversions between
// interfaces across all functions, and, for every resolved concrete vector
// of every generic object, each vector element that is an interface flowing
// into its constraint. Constraint methods are substituted under the vector's
// environment; a method substitution cannot make concrete carries no flow.
func flowEdges(prog *ssa.Program, inst *instantiations, refs *methodReferenceScan, v view) []flowEdge {
	var edges []flowEdge
	seen := map[string]bool{}
	add := func(source, target types.Type, suffix string, env map[*types.TypeParam]types.Type) {
		source, target = types.Unalias(source), types.Unalias(target)
		key := types.TypeString(source, nil) + "\x00" + types.TypeString(target, nil) + "\x00" + suffix
		if seen[key] {
			return
		}
		seen[key] = true
		if edge, ok := buildEdge(prog.Fset, refs, source, target, env); ok {
			edges = append(edges, edge)
		}
	}

	for fn := range ssautil.AllFunctions(prog) {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				switch conv := instr.(type) {
				case *ssa.ChangeInterface:
					if interfaceEndpoint(conv.X.Type()) && interfaceEndpoint(conv.Type()) &&
						v.admitsInstruction(prog.Fset, fn, instr) {
						add(conv.X.Type(), conv.Type(), "", nil)
					}
				case *ssa.ChangeType:
					if interfaceEndpoint(conv.X.Type()) && interfaceEndpoint(conv.Type()) &&
						v.admitsInstruction(prog.Fset, fn, instr) {
						add(conv.X.Type(), conv.Type(), "", nil)
					}
				}
			}
		}
	}

	for obj, vectors := range inst.all {
		params := typeParams(obj)
		if params == nil {
			continue
		}
		for _, vector := range vectors {
			if len(vector) != params.Len() {
				continue
			}
			env := environment(slices.Collect(params.TypeParams()), vector)
			for i, argument := range vector {
				if interfaceEndpoint(argument) {
					add(argument, params.At(i).Constraint(), vectorKey(vector), env)
				}
			}
		}
	}

	return edges
}

// buildEdge resolves an edge's target methods once, so propagation loops over
// prepared work.
func buildEdge(
	fset *token.FileSet,
	refs *methodReferenceScan,
	source, target types.Type,
	env map[*types.TypeParam]types.Type,
) (flowEdge, bool) {
	iface, ok := target.Underlying().(*types.Interface)
	if !ok || iface.NumMethods() == 0 {
		return flowEdge{}, false
	}
	methods := make([]flowMethod, 0, iface.NumMethods())
	for method := range iface.Methods() {
		sig, substituted := substituteSignature(method.Signature(), env)
		if !substituted {
			continue
		}
		key := declKey(position(fset, method.Pos()))
		_, inScan := refs.interfaceMethods[key]
		methods = append(methods, flowMethod{decl: method, sig: sig, key: key, inScan: inScan})
	}
	if len(methods) == 0 {
		return flowEdge{}, false
	}
	return flowEdge{source: types.NewMethodSet(source), methods: methods}, true
}

// interfaceEndpoint reports whether t can be the endpoint of a flow edge:
// an interface that is not a type parameter.
func interfaceEndpoint(t types.Type) bool {
	t = types.Unalias(t)
	if _, isParam := t.(*types.TypeParam); isParam {
		return false
	}
	return types.IsInterface(t)
}
