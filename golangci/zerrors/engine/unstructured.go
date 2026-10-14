package engine

import (
	"cmp"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"
)

// unstructuredReturns reports each return of a judged function that can carry
// an unstructured error, naming one witness. A return whose every unstructured
// origin comes through a call to another judged function draws nothing:
// that function's own return carries the finding. A wrapping conditional
// on the function's parameters is structured, since they are its caller's own
// errors.
func (a *analysis) unstructuredReturns() ([]Finding, error) {
	judged, err := a.judgedFunctions()
	if err != nil {
		return nil, err
	}
	roots := make([]*types.Func, 0, len(judged))
	for fn := range judged {
		roots = append(roots, fn)
	}
	slices.SortFunc(roots, func(x, y *types.Func) int { return cmp.Compare(x.Pos(), y.Pos()) })
	bodies := make([]*ssa.Function, 0, len(roots))
	for _, fn := range roots {
		bodies = append(bodies, a.idx.declared[fn])
	}

	t := newTracer(a, judged)
	t.settle(bodies)

	var findings []Finding
	for i, fn := range roots {
		for _, ret := range t.summary(bodies[i]).returns {
			if !ret.ret.Pos().IsValid() {
				// The return the builder adds for a recovered panic is no statement
				// of the function's, and what it returns is what the statements leave
				// in the results.
				continue
			}
			for _, result := range ret.results {
				if w := result.produced; w != nil {
					findings = append(findings, a.unstructured(fn, bodies[i], ret.ret, w))
					break
				}
			}
		}
	}
	return findings, nil
}

func (a *analysis) unstructured(fn *types.Func, body *ssa.Function, ret *ssa.Return, w *witness) Finding {
	message := funcName(fn) + " can return an unstructured error: " + w.what
	if w.where != body || w.initializes != "" {
		message += " in " + place(w, fn.Pkg())
	}
	return Finding{
		Pos:     a.module.Fset.PositionFor(ret.Pos(), false),
		Rule:    RuleUnstructuredReturn,
		Subject: funcName(fn),
		Message: message,
	}
}
