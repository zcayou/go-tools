package engine

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// renderedOrigin counts as a value's source a String called on a type
// a caller could carry instead: exported, defined and declared in an API
// package, so a caller can name it; comparable, so carrying it gives the caller
// ==; and no enum, whose String is already a member's name, literal-field's
// to judge. A constant is no rendering.
type renderedOrigin struct {
	a     *analysis
	enums map[*types.Named]bool
}

func (renderedOrigin) constants() bool {
	return false
}

// instances leaves a generic body's stores to its instances, where the type
// whose String is called is known.
func (renderedOrigin) instances() bool {
	return true
}

func (o renderedOrigin) result(callee *ssa.Function) (types.Type, bool) {
	sig := callee.Signature
	if sig.Recv() == nil || sig.Params().Len() != 0 || sig.Results().Len() != 1 ||
		!types.Identical(sig.Results().At(0).Type(), types.Typ[types.String]) {
		return nil, false
	}
	method := callee
	if origin := callee.Origin(); origin != nil {
		method = origin
	}
	if method.Name() != "String" {
		return nil, false
	}
	named, ok := types.Unalias(pointee(sig.Recv().Type())).(*types.Named)
	if !ok {
		return nil, false
	}
	obj := named.Obj()
	if !obj.Exported() || obj.Pkg() == nil || !o.a.api[obj.Pkg().Path()] || o.enums[named.Origin()] ||
		types.IsInterface(named) || !types.Comparable(named) {
		return nil, false
	}
	return named, true
}

// renderedFields reports each string field of a module error type that has
// at least one store and only ever holds the String of one type a caller could
// carry instead.
func (a *analysis) renderedFields() []Finding {
	f := newFlow(a, renderedOrigin{a: a, enums: a.apiEnums()})

	var candidates []*types.Var
	owners := map[*types.Var]*types.Named{}
	for _, named := range a.idx.types {
		if errorNamed(named) == nil {
			continue
		}
		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for field := range st.Fields() {
			if types.Identical(field.Type(), types.Typ[types.String]) && f.stored(field) {
				candidates = append(candidates, field)
				owners[field] = named
				f.build(node{field: field})
			}
		}
	}
	lost := f.settle()
	var findings []Finding
	for _, field := range candidates {
		if lost[node{field: field}] {
			continue
		}
		rendered := f.rendered(node{field: field})
		if len(rendered) != 1 {
			continue
		}
		owner := typeName(owners[field]) + "." + field.Name()
		findings = append(findings, Finding{
			Pos:     a.module.Fset.PositionFor(field.Pos(), false),
			Rule:    RuleRenderedField,
			Subject: owner,
			Message: owner + " holds only the String of " + types.TypeString(rendered[0], nil) + "; carry " +
				types.TypeString(rendered[0], packageName),
		})
	}
	return findings
}
