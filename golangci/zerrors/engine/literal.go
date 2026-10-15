package engine

import (
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/zcayou/go-tools/golangci/internal/enum"
)

// literalOrigin counts a constant as a value's source, and the name
// of an exported API enum's member, which is one.
type literalOrigin struct {
	// enums holds the exported enums of API packages, whose String() is a member's
	// name.
	enums map[*types.Named]bool
}

func (literalOrigin) constants() bool {
	return true
}

func (literalOrigin) instances() bool {
	return false
}

func (o literalOrigin) result(callee *ssa.Function) (types.Type, bool) {
	return nil, o.memberName(callee)
}

// memberName reports whether fn is the String method naming an exported API
// enum's members. An internal type's String is a display string, and is not
// followed.
func (o literalOrigin) memberName(fn *ssa.Function) bool {
	obj, ok := fn.Object().(*types.Func)
	if !ok || obj.Signature().Recv() == nil {
		return false
	}
	named, ok := types.Unalias(pointee(obj.Signature().Recv().Type())).(*types.Named)
	return ok && o.enums[named] && enum.Name(named) == obj
}

// literalFields reports each string or []string field of a module error type
// that has at least one store and only ever holds constants.
func (a *analysis) literalFields() []Finding {
	o := literalOrigin{enums: a.apiEnums()}
	f := newFlow(a, o)

	var findings []Finding
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
			if textual(field.Type()) && f.stored(field) {
				candidates = append(candidates, field)
				owners[field] = named
				f.build(node{field: field})
			}
		}
	}
	lost := f.settle()
	holding := newHolding(a, o)
	for _, field := range candidates {
		if lost[node{field: field}] {
			continue
		}
		owner := typeName(owners[field]) + "." + field.Name()
		message := owner + " only ever holds constants; give it an enum type"
		// One value is no choice an enum could name: the field says nothing.
		if held := holding.field(field); held.known && !held.many {
			message = owner + " only ever holds " + held.one + ", so it carries no information; remove it"
		}
		findings = append(findings, Finding{
			Pos:     a.module.Fset.PositionFor(field.Pos(), false),
			Rule:    RuleLiteralField,
			Subject: owner,
			Message: message,
		})
	}
	return findings
}

// textual reports whether a field's type is exactly string or []string.
// A named string type is already a typing decision, which zenum governs.
func textual(t types.Type) bool {
	if slice, ok := t.(*types.Slice); ok {
		t = slice.Elem()
	}
	return types.Identical(t, types.Typ[types.String])
}
