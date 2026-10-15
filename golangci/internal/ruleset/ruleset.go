// Package ruleset decides which of a linter's rules run.
//
// Every linter's settings carry the same three keys, in the shape golangci-lint
// gives its own linters block:
//
//	default: standard     # standard, all or none; absent means standard
//	enable: [rule, ...]
//	disable: [rule, ...]
//
// The enabled set is the default set, plus enable, minus disable. standard
// is every rule but the ones the linter makes opt-in, all is every rule,
// and none is no rule.
//
// A selection that changes nothing does not mean what it says, so each of these
// is rejected: a default other than the three values, a name that is no rule
// of the linter, a rule named twice in one list or once in each, an enable
// of a rule the default already enables or a disable of one it does not,
// an explicitly empty list, and a selection enabling no rule of a linter whose
// findings all come from rules.
package ruleset

import "slices"

// The values default takes.
const (
	// Standard is every rule but the opt-in ones.
	Standard = "standard"
	// All is every rule.
	All = "all"
	// None is no rule.
	None = "none"
)

// Selection is the part of a linter's settings that chooses its rules.
// A linter's Settings embeds it, so its keys sit beside the linter's own.
type Selection struct {
	// Default is the set enable and disable start from: standard, all or none.
	// An absent key is standard.
	Default *string `json:"default"`

	// Enable names rules to run that the default leaves out.
	Enable []string `json:"enable"`

	// Disable names rules not to run that the default includes.
	Disable []string `json:"disable"`
}

// Rule is a linter's rule enum, whose String is the token a selection names
// it by.
type Rule interface {
	comparable
	String() string
}

// Catalog is what a linter offers to select from.
type Catalog[R Rule] struct {
	// Rules is every rule the linter has.
	Rules []R

	// OptIn is the rules standard leaves out.
	OptIn []R

	// Beside says the linter reports findings no rule names, so a selection
	// enabling no rule still leaves it something to report.
	Beside bool
}

// Resolve returns the rules s enables, or an [*Error] saying why s means
// nothing.
func (c Catalog[R]) Resolve(s Selection) (Enabled[R], error) {
	byName := make(map[string]R, len(c.Rules))
	for _, rule := range c.Rules {
		byName[rule.String()] = rule
	}

	enabled, err := c.defaults(s.Default)
	if err != nil {
		return Enabled[R]{}, err
	}
	enable, err := named(KeyEnable, s.Enable, byName)
	if err != nil {
		return Enabled[R]{}, err
	}
	disable, err := named(KeyDisable, s.Disable, byName)
	if err != nil {
		return Enabled[R]{}, err
	}

	for _, rule := range enable {
		if slices.Contains(disable, rule) {
			return Enabled[R]{}, &Error{Key: KeyEnable, Problem: ProblemEnabledAndDisabled, Value: rule.String()}
		}
		if enabled[rule] {
			return Enabled[R]{}, &Error{Key: KeyEnable, Problem: ProblemAlreadyEnabled, Value: rule.String()}
		}
		enabled[rule] = true
	}
	for _, rule := range disable {
		if !enabled[rule] {
			return Enabled[R]{}, &Error{Key: KeyDisable, Problem: ProblemNotEnabled, Value: rule.String()}
		}
		delete(enabled, rule)
	}

	if len(enabled) == 0 && !c.Beside {
		return Enabled[R]{}, &Error{Key: KeyDefault, Problem: ProblemNoRuleEnabled}
	}
	return Enabled[R]{rules: enabled}, nil
}

// defaults returns the set a default value names, as a set of its own
// the caller may change.
func (c Catalog[R]) defaults(value *string) (map[R]bool, error) {
	set := make(map[R]bool, len(c.Rules))
	name := Standard
	if value != nil {
		name = *value
	}
	switch name {
	case Standard:
		for _, rule := range c.Rules {
			if !slices.Contains(c.OptIn, rule) {
				set[rule] = true
			}
		}
	case All:
		for _, rule := range c.Rules {
			set[rule] = true
		}
	case None:
	default:
		return nil, &Error{Key: KeyDefault, Problem: ProblemUnknownDefault, Value: name}
	}
	return set, nil
}

// named resolves the rule names a list holds. A list written out empty says
// nothing, and is rejected rather than read as absent.
func named[R Rule](key Key, names []string, byName map[string]R) ([]R, error) {
	if names != nil && len(names) == 0 {
		return nil, &Error{Key: key, Problem: ProblemEmptyList}
	}
	rules := make([]R, 0, len(names))
	for _, name := range names {
		rule, ok := byName[name]
		if !ok {
			return nil, &Error{Key: key, Problem: ProblemUnknownRule, Value: name}
		}
		if slices.Contains(rules, rule) {
			return nil, &Error{Key: key, Problem: ProblemRepeatedRule, Value: name}
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// Enabled is the set of rules a selection runs. The zero value enables none.
type Enabled[R Rule] struct {
	rules map[R]bool
}

// Has reports whether rule runs.
func (e Enabled[R]) Has(rule R) bool {
	return e.rules[rule]
}

// Any reports whether any of rules runs.
func (e Enabled[R]) Any(rules ...R) bool {
	return slices.ContainsFunc(rules, e.Has)
}

// Disabled returns the rules of c that do not run, in c's order, or nil when
// every one does.
func (e Enabled[R]) Disabled(c Catalog[R]) []R {
	var off []R
	for _, rule := range c.Rules {
		if !e.Has(rule) {
			off = append(off, rule)
		}
	}
	return off
}
