package engine

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/internal/enum"
)

// enumType is one enum the rules judge: a module-declared integer or string
// type with members, not taken out by open-enums.
type enumType struct {
	*enum.Enum

	// pkg is the module package declaring it, whose syntax its String is read
	// from.
	pkg *packages.Package

	// qualified is its import/path.Name, how configuration names it.
	qualified string

	// tokens are what its members render as.
	tokens enum.Tokens

	// visible reports that a reader outside its package can meet its values.
	visible bool

	// zeroDefault reports that zero-defaults lists it.
	zeroDefault bool

	// orderedEnum reports that ordered-enums lists it, and orderedOutside
	// that something outside its package orders it.
	orderedEnum    bool
	orderedOutside bool
}

// display is the enum as a message names it: qualified by its package's
// name, as source code spells it.
func (v *enumType) display() string {
	return v.pkg.Name + "." + v.Type.Obj().Name()
}

// zero returns the first member whose value is zero, or nil.
func (v *enumType) zero() *types.Const {
	for _, member := range v.Members {
		if isZero(member.Val()) {
			return member
		}
	}
	return nil
}

// member returns the first member holding value, or nil when none does.
func (v *enumType) member(value constant.Value) *types.Const {
	if !v.Member(value) {
		return nil
	}
	for _, member := range v.Members {
		if constant.Compare(member.Val(), token.EQL, value) {
			return member
		}
	}
	return nil
}

func isZero(value constant.Value) bool {
	switch value.Kind() {
	case constant.String:
		return constant.StringVal(value) == ""
	case constant.Int, constant.Float, constant.Complex:
		return constant.Sign(value) == 0
	default:
		return false
	}
}

// exceptions applies the configured exceptions, returning an error for each
// one that suppresses nothing. open-enums goes first, so that an entry
// elsewhere naming a type it opened names no enum.
func (a *analysis) exceptions(cfg Config) []error {
	var stale []error
	opened := map[*enumType]bool{}
	for _, name := range cfg.OpenEnums {
		v := a.lookup(name)
		if v == nil {
			stale = append(stale, &ExceptionError{Setting: SettingOpenEnums, Fault: FaultNotEnum, Type: name})
			continue
		}
		opened[v] = true
	}
	a.enums = slices.DeleteFunc(a.enums, func(v *enumType) bool { return opened[v] })
	for _, v := range a.enums {
		a.byType[v.Type] = v
		if v.tokens.Method != nil {
			a.byName[v.tokens.Method] = v
		}
	}

	for _, name := range cfg.ZeroDefaults {
		v := a.lookup(name)
		switch {
		case v == nil:
			stale = append(stale, &ExceptionError{Setting: SettingZeroDefaults, Fault: FaultNotEnum, Type: name})
		case v.zero() == nil:
			stale = append(stale, &ExceptionError{Setting: SettingZeroDefaults, Fault: FaultNoZeroMember, Type: name})
		default:
			v.zeroDefault = true
		}
	}
	for _, name := range cfg.OrderedEnums {
		if v := a.lookup(name); v != nil {
			v.orderedEnum = true
		} else {
			stale = append(stale, &ExceptionError{Setting: SettingOrderedEnums, Fault: FaultNotEnum, Type: name})
		}
	}
	return stale
}

func (a *analysis) lookup(qualified string) *enumType {
	for _, v := range a.enums {
		if v.qualified == qualified {
			return v
		}
	}
	return nil
}

// enumOf returns the enum t is, or nil.
func (a *analysis) enumOf(t types.Type) *enumType {
	if t == nil {
		return nil
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return nil
	}
	return a.byType[named]
}

// declarations judges each enum's declaration: its underlying type, its name,
// and its members.
func (a *analysis) declarations() {
	for _, v := range a.enums {
		if v.Textual() {
			a.report(v.Type.Obj().Pos(), RuleStringEnum, v, fmt.Sprintf(
				"%s has a string underlying type, so its tokens are its identities; make it ordinal and name it "+
					"with String", v.display()))
		}
		if v.Ordinal() {
			a.names(v)
		}
		if zero := v.zero(); zero != nil && v.visible && !v.zeroDefault {
			a.report(zero.Pos(), RuleZeroMember, v, fmt.Sprintf(
				"%s is %s's zero value, which an unset field also holds; start the members above zero",
				zero.Name(), v.display()))
		}
	}
}

// names judges an ordinal enum's String: whether a visible one has a readable
// one, and whether a readable one names every member value once.
func (a *analysis) names(v *enumType) {
	switch {
	case v.tokens.Method == nil:
		if v.visible {
			a.report(v.Type.Obj().Pos(), RuleUnnamed, v, fmt.Sprintf(
				"%s is visible outside its package but has no String() string on a value receiver", v.display()))
		}
		return
	case !v.tokens.Readable:
		if v.visible {
			a.report(v.tokens.Method.Pos(), RuleUnreadableName, v, fmt.Sprintf(
				"%s's String is in no readable form, so the names it publishes go unverified", v.display()))
		}
		return
	}

	firstWith := map[string]*types.Const{}
	for _, value := range v.Values() {
		member := v.member(value)
		rendering, ok := v.tokens.Token(value)
		switch {
		case !ok:
			a.report(member.Pos(), RuleAmbiguousName, v, fmt.Sprintf(
				"%s's String gives %s no token", v.display(), member.Name()))
		case rendering == "":
			a.report(member.Pos(), RuleAmbiguousName, v, fmt.Sprintf(
				"%s's String gives %s an empty token", v.display(), member.Name()))
		case firstWith[rendering] != nil:
			a.report(member.Pos(), RuleAmbiguousName, v, fmt.Sprintf(
				"%s's String gives %s the token %q, as it does %s", v.display(), member.Name(), rendering,
				firstWith[rendering].Name()))
		default:
			firstWith[rendering] = member
		}
	}
}
