package spec

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"
)

// identity is one errors call an identity boolean holds the answer of.
type identity struct {
	// function is the errors function answering: Is, As or AsType.
	function string

	// negated reports whether the boolean holds the negation of its answer.
	negated bool
}

// identityResult is what [passChecker.variableIdentity] found of one variable.
type identityResult struct {
	identities []identity
	identity   bool
}

// identityOf returns the errors calls expr is an identity boolean of:
//   - a call of errors.Is or errors.As;
//   - a local variable every write to which is one, or the second result
//     of errors.AsType;
//   - the negation of one, or the conjunction or disjunction of two.
func (p *passChecker) identityOf(expr ast.Expr) ([]identity, bool) {
	switch e := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		if name, ok := p.refers(e.Fun, errorsPath); ok && (name == "Is" || name == "As") {
			return []identity{{function: name}}, true
		}
	case *ast.Ident:
		return p.variableIdentity(e)
	case *ast.UnaryExpr:
		if e.Op != token.NOT {
			break
		}
		found, ok := p.identityOf(e.X)
		negated := make([]identity, len(found))
		for i, id := range found {
			negated[i] = identity{function: id.function, negated: !id.negated}
		}
		return negated, ok
	case *ast.BinaryExpr:
		if e.Op != token.LAND && e.Op != token.LOR {
			break
		}
		x, xok := p.identityOf(e.X)
		y, yok := p.identityOf(e.Y)
		if !xok || !yok {
			return nil, false
		}
		return slices.Concat(x, y), true
	}
	return nil, false
}

// variableIdentity reports whether a variable declared in a function
// is an identity boolean: every write to it, and there is one, is one, or binds
// the second result of errors.AsType.
func (p *passChecker) variableIdentity(id *ast.Ident) ([]identity, bool) {
	key, v := p.variableOf(id)
	if v == nil || v.fn == nil {
		return nil, false
	}
	if found, ok := p.identities[key]; ok {
		return found.identities, found.identity
	}
	// A variable written from itself reads as none while it is being read.
	p.identities[key] = identityResult{}

	found := identityResult{identity: len(v.writes) > 0}
	for _, w := range v.writes {
		var ids []identity
		ok := false
		switch {
		case w.value == nil:
		case w.index < 0:
			ids, ok = p.identityOf(w.value)
		case w.index == 1:
			if call, isCall := ast.Unparen(w.value).(*ast.CallExpr); isCall {
				name, refers := p.refers(call.Fun, errorsPath)
				ids, ok = []identity{{function: "AsType"}}, refers && name == "AsType"
			}
		}
		if !ok {
			found = identityResult{}
			break
		}
		found.identities = append(found.identities, ids...)
	}
	p.identities[key] = found
	return found.identities, found.identity
}

// heldBy returns the values an assertion comparing a boolean with expr holds
// it to: the constant's, or either when expr is a bool no constant gives.
// It reports false when expr is no bool.
func (p *passChecker) heldBy(expr ast.Expr) ([]bool, bool) {
	tv, ok := p.pass.TypesInfo.Types[expr]
	if !ok || tv.Type == nil {
		return nil, false
	}
	basic, ok := tv.Type.Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsBoolean == 0 {
		return nil, false
	}
	if tv.Value != nil && tv.Value.Kind() == constant.Bool {
		return []bool{constant.BoolVal(tv.Value)}, true
	}
	return []bool{true, false}, true
}

// gomegaIdentity judges a Gomega matcher holding its actual value to held:
// an assertion of an errors.Is boolean, which MatchError would make
// with the error in its failure. Gomega has no matcher performing errors.As, so
// an As boolean is not judged.
func (f *fileChecker) gomegaIdentity(call *ast.CallExpr, stack []ast.Node, held []bool) {
	site := f.matcherContext(call, stack)
	if site.actual == nil || site.transformed {
		return
	}
	ids, ok := f.identityOf(site.actual)
	if !ok || slices.ContainsFunc(ids, func(id identity) bool { return id.function != "Is" }) {
		return
	}
	if site.negated {
		held = negations(held)
	}
	f.report(call.Pos(), RuleBooleanIdentity, identityMessage(ids, held, func(affirms bool, _ string) string {
		if affirms {
			return "MatchError(target)"
		}
		return "NotTo(MatchError(target))"
	}))
}

// testifyIdentity judges a testify assertion given an identity boolean
// as a value under test, which ErrorIs or ErrorAs would make with the error
// chain in its failure.
func (f *fileChecker) testifyIdentity(call *ast.CallExpr, name string, values []ast.Expr) {
	var held []bool
	var ids []identity
	switch name {
	case "True", "False":
		if len(values) == 0 {
			return
		}
		found, ok := f.identityOf(values[0])
		if !ok {
			return
		}
		ids, held = found, []bool{name == "True"}
	case "Equal", "EqualValues", "Exactly", "NotEqual", "NotEqualValues":
		if len(values) != 2 {
			return
		}
		i := slices.IndexFunc(values, func(value ast.Expr) bool {
			_, ok := f.identityOf(value)
			return ok
		})
		if i < 0 {
			return
		}
		ids, _ = f.identityOf(values[i])
		var ok bool
		if held, ok = f.heldBy(values[1-i]); !ok {
			held = []bool{true, false}
		}
		if strings.HasPrefix(name, "Not") {
			held = negations(held)
		}
	default:
		return
	}
	f.report(call.Pos(), RuleBooleanIdentity, identityMessage(ids, held, func(affirms bool, function string) string {
		remedy := "ErrorAs"
		if function == "Is" {
			remedy = "ErrorIs"
		}
		if !affirms {
			remedy = "Not" + remedy
		}
		return remedy
	}))
}

// negations returns the negation of each value held.
func negations(held []bool) []bool {
	negated := make([]bool, len(held))
	for i, value := range held {
		negated[i] = !value
	}
	return negated
}

// identityMessage says which errors calls an assertion holds to the values
// held, and what to assert instead: remedy names the assertion for a call
// that the spec affirms found its target, or denies.
func identityMessage(ids []identity, held []bool, remedy func(affirms bool, function string) string) string {
	var functions, remedies []string
	for _, id := range ids {
		if name := "errors." + id.function; !slices.Contains(functions, name) {
			functions = append(functions, name)
		}
		for _, value := range held {
			if r := remedy(value != id.negated, id.function); !slices.Contains(remedies, r) {
				remedies = append(remedies, r)
			}
		}
	}
	found := " found its target"
	if len(functions) > 1 {
		found = " found their targets"
	}
	return "asserts whether " + strings.Join(functions, " and ") + found + ", so a failure shows no error; use " +
		strings.Join(remedies, " or ")
}
