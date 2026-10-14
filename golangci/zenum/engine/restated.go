package engine

import (
	"fmt"
	"maps"
	"slices"
)

// restatements reports each enum whose tokens equal, contain or are contained
// in an earlier one's, both holding at least two: one concept spelled twice,
// which drifts apart the first time one of them is renamed. Enums are ordered
// by import path and then name, and the finding sits on the later of the two,
// naming the earlier. An unreadable String publishes tokens nobody can check,
// and is compared with nothing.
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

// tokenSet returns the distinct tokens an enum's members render as, or nil when
// they cannot be read.
func tokenSet(v *enumType) map[string]bool {
	if !v.tokens.Readable {
		return nil
	}
	set := map[string]bool{}
	for _, value := range v.Values() {
		if token, ok := v.tokens.Token(value); ok {
			set[token] = true
		}
	}
	return set
}

func overlaps(x, y map[string]bool) bool {
	return contains(x, y) || contains(y, x)
}

// contains reports whether every token of inner is one of outer's.
func contains(outer, inner map[string]bool) bool {
	for token := range inner {
		if !outer[token] {
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
