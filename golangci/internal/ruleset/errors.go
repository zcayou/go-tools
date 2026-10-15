package ruleset

import (
	"fmt"
	"strconv"
)

// Error is a selection that means nothing. A linter returns it as the cause
// of its own configuration error, naming the key it maps to.
type Error struct {
	// Key is the setting at fault.
	Key Key

	// Problem is what is wrong with it.
	Problem Problem

	// Value is the default or the rule name at fault, or empty when the fault
	// is with no one value.
	Value string
}

func (e *Error) Error() string {
	switch e.Problem {
	case ProblemUnknownDefault:
		return fmt.Sprintf("%s: %q is none of %s, %s and %s", e.Key, e.Value, Standard, All, None)
	case ProblemUnknownRule:
		return fmt.Sprintf("%s: %q is no rule of this linter", e.Key, e.Value)
	case ProblemRepeatedRule:
		return fmt.Sprintf("%s: %q is listed twice", e.Key, e.Value)
	case ProblemEnabledAndDisabled:
		return fmt.Sprintf("%s: %q is also listed under %s", e.Key, e.Value, KeyDisable)
	case ProblemAlreadyEnabled:
		return fmt.Sprintf("%s: %q is already enabled by %s", e.Key, e.Value, KeyDefault)
	case ProblemNotEnabled:
		return fmt.Sprintf("%s: %q is not enabled by %s, so there is nothing to disable", e.Key, e.Value, KeyDefault)
	case ProblemEmptyList:
		return fmt.Sprintf("%s: an empty list names no rule; leave the key out", e.Key)
	case ProblemNoRuleEnabled:
		return "the selection enables no rule; disable the linter instead"
	default:
		return fmt.Sprintf("%s: %s", e.Key, e.Problem)
	}
}

// Key is a setting of [Selection].
type Key int

const (
	// KeyDefault is default.
	KeyDefault Key = iota + 1
	// KeyEnable is enable.
	KeyEnable
	// KeyDisable is disable.
	KeyDisable
)

// String is the key as the settings spell it.
func (k Key) String() string {
	switch k {
	case KeyDefault:
		return "default"
	case KeyEnable:
		return "enable"
	case KeyDisable:
		return "disable"
	default:
		return "Key(" + strconv.Itoa(int(k)) + ")"
	}
}

// Problem is why a selection means nothing.
type Problem int

const (
	// ProblemUnknownDefault is a default other than standard, all and none.
	ProblemUnknownDefault Problem = iota + 1
	// ProblemUnknownRule is a name that is no rule of the linter.
	ProblemUnknownRule
	// ProblemRepeatedRule is a rule listed twice in one list.
	ProblemRepeatedRule
	// ProblemEnabledAndDisabled is a rule listed under both enable and disable.
	ProblemEnabledAndDisabled
	// ProblemAlreadyEnabled is an enable of a rule the default enables.
	ProblemAlreadyEnabled
	// ProblemNotEnabled is a disable of a rule the default leaves out.
	ProblemNotEnabled
	// ProblemEmptyList is a list written out empty.
	ProblemEmptyList
	// ProblemNoRuleEnabled is a selection that leaves a linter whose findings
	// all come from rules with none.
	ProblemNoRuleEnabled
)

// String is the problem's token.
func (p Problem) String() string {
	switch p {
	case ProblemUnknownDefault:
		return "unknown-default"
	case ProblemUnknownRule:
		return "unknown-rule"
	case ProblemRepeatedRule:
		return "repeated-rule"
	case ProblemEnabledAndDisabled:
		return "enabled-and-disabled"
	case ProblemAlreadyEnabled:
		return "already-enabled"
	case ProblemNotEnabled:
		return "not-enabled"
	case ProblemEmptyList:
		return "empty-list"
	case ProblemNoRuleEnabled:
		return "no-rule-enabled"
	default:
		return "Problem(" + strconv.Itoa(int(p)) + ")"
	}
}
