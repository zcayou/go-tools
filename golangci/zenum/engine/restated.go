package engine

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"iter"
	"maps"
	"slices"
)

// restatements reports each enum whose tokens equal, contain or are contained
// in an earlier one's, both holding at least two: one concept spelled twice,
// which drifts apart the first time one of them is renamed. Enums are ordered
// by import path and then name, and the finding sits on the later of the two,
// naming the earlier. An unreadable String publishes tokens nobody can check,
// and is compared with nothing. It then reports each enum no reader outside
// its package meets that a bridge maps member for member onto another,
// as [analysis.bridged] says.
//
// It returns an error for each related pair that is not a restatement.
func (a *analysis) restatements(related []Pair) []error {
	tokens := make(map[*enumType]map[string]bool, len(a.enums))
	for _, v := range a.enums {
		if set := tokenSet(v); len(set) >= 2 {
			tokens[v] = set
		}
	}

	restated := map[[2]*enumType]bool{}
	for j, later := range a.enums {
		for _, earlier := range a.enums[:j] {
			if tokens[later] == nil || tokens[earlier] == nil || !overlaps(tokens[earlier], tokens[later]) {
				continue
			}
			restated[[2]*enumType{earlier, later}] = true
			restated[[2]*enumType{later, earlier}] = true
			if relatedPair(related, earlier, later) {
				continue
			}
			a.report(later.Type.Obj().Pos(), RuleRestated, later, fmt.Sprintf(
				"%s %s the tokens of %s", later.display(), relation(tokens[earlier], tokens[later]), earlier.qualified))
		}
	}
	a.bridged(related, restated)

	var stale []error
	for _, pair := range related {
		first, second := a.lookup(pair.First), a.lookup(pair.Second)
		switch {
		case first == nil:
			stale = append(stale, &ExceptionError{
				Setting: SettingRelatedEnums, Fault: FaultNotEnum, Type: pair.First, Related: pair.Second,
			})
		case second == nil:
			stale = append(stale, &ExceptionError{
				Setting: SettingRelatedEnums, Fault: FaultNotEnum, Type: pair.Second, Related: pair.First,
			})
		case !restated[[2]*enumType{first, second}]:
			stale = append(stale, &ExceptionError{
				Setting: SettingRelatedEnums, Fault: FaultNotRestated, Type: pair.First, Related: pair.Second,
			})
		}
	}
	return stale
}

// bridged reports each enum no reader outside its package meets, holding
// at least two member values, that a bridge pairs one to one, every member
// value of it, with members of another enum: a private copy of a vocabulary
// that needs no tokens to be one, since the mapping says the two are one
// concept spelled twice. Two visible enums are each a vocabulary readers meet,
// as a wire enum and a domain enum are, and only their tokens decide them. When
// both of a bridge's enums are copies, the later in import-path-then-name order
// is reported, as tokens place it. A pair restated already is reported once,
// and restated records each pair a bridge restates.
func (a *analysis) bridged(related []Pair, restated map[[2]*enumType]bool) {
	order := make(map[*enumType]int, len(a.enums))
	for i, v := range a.enums {
		order[v] = i
	}
	for _, b := range a.bridges() {
		if !oneToOne(b.pairs) {
			continue
		}
		var copies [][2]*enumType
		for side, v := range [2]*enumType{b.from, b.to} {
			if !v.visible && len(v.Values()) >= 2 && covers(v, b.pairs, side) {
				copies = append(copies, [2]*enumType{v, [2]*enumType{b.to, b.from}[side]})
			}
		}
		if len(copies) == 2 {
			later := copies[1]
			if order[copies[0][0]] > order[later[0]] {
				later = copies[0]
			}
			copies = [][2]*enumType{later}
		}
		for _, pair := range copies {
			copied, of := pair[0], pair[1]
			if restated[pair] {
				continue
			}
			restated[pair] = true
			restated[[2]*enumType{of, copied}] = true
			if relatedPair(related, copied, of) {
				continue
			}
			a.report(copied.Type.Obj().Pos(), RuleRestated, copied, fmt.Sprintf(
				"%s restates %s member for member, as %s maps them; use %s",
				copied.Type.Obj().Name(), of.display(), b.site, of.display()))
		}
	}
}

// bridge is code pairing the members of two enums: a switch over one whose
// every case lists members of it and returns or assigns one member
// of the other, or a map literal from one to the other.
type bridge struct {
	from, to *enumType

	// pairs holds each member value of from the bridge pairs with one of to.
	pairs [][2]constant.Value

	// site names the function or variable the bridge is written in.
	site string
}

// bridges returns the bridges the module's packages write. A test file writes
// none.
func (a *analysis) bridges() []bridge {
	var found []bridge
	for _, pkg := range a.pkgs {
		info := pkg.TypesInfo
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				for site, node := range sites(decl) {
					ast.Inspect(node, func(n ast.Node) bool {
						var b bridge
						var ok bool
						switch n := n.(type) {
						case *ast.SwitchStmt:
							b, ok = a.switchBridge(info, n)
						case *ast.CompositeLit:
							b, ok = a.mapBridge(info, n)
						}
						if ok {
							b.site = site
							found = append(found, b)
						}
						return true
					})
				}
			}
		}
	}
	return found
}

// sites yields what a declaration holds by the name a bridge in it is known by:
// a function or method as itself, and each variable spec by its first name.
func sites(decl ast.Decl) iter.Seq2[string, ast.Node] {
	return func(yield func(string, ast.Node) bool) {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			name := decl.Name.Name
			if decl.Recv != nil && len(decl.Recv.List) == 1 {
				if recv := receiverName(decl.Recv.List[0].Type); recv != "" {
					name = recv + "." + name
				}
			}
			yield(name, decl)
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				if spec, ok := spec.(*ast.ValueSpec); ok && len(spec.Names) > 0 {
					if !yield(spec.Names[0].Name, spec) {
						return
					}
				}
			}
		}
	}
}

// receiverName returns the type name a method's receiver is written with.
func receiverName(expr ast.Expr) string {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.StarExpr:
		return receiverName(expr.X)
	case *ast.IndexExpr:
		return receiverName(expr.X)
	case *ast.IndexListExpr:
		return receiverName(expr.X)
	case *ast.Ident:
		return expr.Name
	}
	return ""
}

// switchBridge returns the bridge a switch is: one whose tag has enum type,
// whose every case clause lists only members of that enum, and whose every
// such clause's body is one statement giving a member of one other enum —
// a return holding exactly one, or an assignment of one to the variable every
// clause assigns. A default clause pairs nothing.
func (a *analysis) switchBridge(info *types.Info, stmt *ast.SwitchStmt) (bridge, bool) {
	if stmt.Tag == nil {
		return bridge{}, false
	}
	b := bridge{from: a.enumOf(info.TypeOf(stmt.Tag))}
	if b.from == nil {
		return bridge{}, false
	}
	var assigned types.Object
	for _, clause := range stmt.Body.List {
		clause, ok := clause.(*ast.CaseClause)
		if !ok || clause.List == nil {
			continue
		}
		if len(clause.Body) != 1 {
			return bridge{}, false
		}
		to, value, target := a.clauseMember(info, clause.Body[0])
		if to == nil || b.to != nil && (to != b.to || target != assigned) {
			return bridge{}, false
		}
		b.to, assigned = to, target
		for _, expr := range clause.List {
			from, member := a.memberOf(info, expr)
			if from != b.from {
				return bridge{}, false
			}
			b.pairs = append(b.pairs, [2]constant.Value{member, value})
		}
	}
	return b, b.to != nil && b.to != b.from
}

// clauseMember returns the member of an enum a case clause's statement gives,
// and the variable it assigns it to, if it does not return it.
func (a *analysis) clauseMember(info *types.Info, stmt ast.Stmt) (*enumType, constant.Value, types.Object) {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		var to *enumType
		var value constant.Value
		for _, result := range stmt.Results {
			if v, member := a.memberOf(info, result); v != nil {
				if to != nil {
					return nil, nil, nil
				}
				to, value = v, member
			}
		}
		return to, value, nil
	case *ast.AssignStmt:
		if stmt.Tok != token.ASSIGN || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
			return nil, nil, nil
		}
		id, ok := ast.Unparen(stmt.Lhs[0]).(*ast.Ident)
		if !ok || info.Uses[id] == nil {
			return nil, nil, nil
		}
		to, value := a.memberOf(info, stmt.Rhs[0])
		return to, value, info.Uses[id]
	}
	return nil, nil, nil
}

// mapBridge returns the bridge a composite literal of a map type from one enum
// to another is, when its every key and value is a member.
func (a *analysis) mapBridge(info *types.Info, lit *ast.CompositeLit) (bridge, bool) {
	m, ok := types.Unalias(info.TypeOf(lit)).Underlying().(*types.Map)
	if !ok || len(lit.Elts) == 0 {
		return bridge{}, false
	}
	b := bridge{from: a.enumOf(m.Key()), to: a.enumOf(m.Elem())}
	if b.from == nil || b.to == nil || b.from == b.to {
		return bridge{}, false
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return bridge{}, false
		}
		from, key := a.memberOf(info, kv.Key)
		to, value := a.memberOf(info, kv.Value)
		if from != b.from || to != b.to {
			return bridge{}, false
		}
		b.pairs = append(b.pairs, [2]constant.Value{key, value})
	}
	return b, true
}

// oneToOne reports whether no member value on either side of the pairs
// is paired twice.
func oneToOne(pairs [][2]constant.Value) bool {
	var seen [2]map[string]bool
	for side := range seen {
		seen[side] = map[string]bool{}
	}
	for _, pair := range pairs {
		for side, value := range pair {
			if seen[side][value.ExactString()] {
				return false
			}
			seen[side][value.ExactString()] = true
		}
	}
	return true
}

// covers reports whether the pairs hold every member value of v on one side.
func covers(v *enumType, pairs [][2]constant.Value, side int) bool {
	paired := map[string]bool{}
	for _, pair := range pairs {
		paired[pair[side].ExactString()] = true
	}
	for _, value := range v.Values() {
		if !paired[value.ExactString()] {
			return false
		}
	}
	return true
}

// tokenSet returns the distinct tokens an enum's members render as, or nil when
// they cannot be read.
func tokenSet(v *enumType) map[string]bool {
	if !v.tokens.Readable {
		return nil
	}
	set := map[string]bool{}
	for _, value := range v.Values() {
		if rendered, ok := v.tokens.Token(value); ok {
			set[rendered] = true
		}
	}
	return set
}

func overlaps(x, y map[string]bool) bool {
	return contains(x, y) || contains(y, x)
}

// contains reports whether every token of inner is one of outer's.
func contains(outer, inner map[string]bool) bool {
	for rendered := range inner {
		if !outer[rendered] {
			return false
		}
	}
	return true
}

// relation says how later's tokens stand to earlier's.
func relation(earlier, later map[string]bool) string {
	switch {
	case maps.Equal(earlier, later):
		return "restates"
	case contains(later, earlier):
		return "restates and extends"
	default:
		return "restates a subset of"
	}
}

func relatedPair(related []Pair, x, y *enumType) bool {
	return slices.ContainsFunc(related, func(pair Pair) bool {
		return pair.First == x.qualified && pair.Second == y.qualified ||
			pair.First == y.qualified && pair.Second == x.qualified
	})
}
