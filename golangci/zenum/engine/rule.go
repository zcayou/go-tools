package engine

import "fmt"

// Rule is what a [Finding] reports. Its token opens the finding's message,
// which is what makes one rule addressable by an exclusion while the linter
// stays enabled.
type Rule int

const (
	// RuleStringEnum is an enum with a string underlying type.
	RuleStringEnum Rule = iota + 1
	// RuleUnnamed is a visible ordinal enum with no String() string on a value
	// receiver.
	RuleUnnamed
	// RuleUnreadableName is a visible ordinal enum whose String is in no
	// readable form.
	RuleUnreadableName
	// RuleAmbiguousName is a readable String giving a member value no token,
	// an empty one, or one another member value also gets.
	RuleAmbiguousName
	// RuleZeroMember is a visible enum with a member at zero.
	RuleZeroMember
	// RuleRestated is an enum whose tokens equal, contain or are contained
	// in an earlier one's.
	RuleRestated
	// RuleTokenDecision is a text decision on a rendered token, or a comparison
	// of an enum value with a constant no member holds.
	RuleTokenDecision
	// RuleForgedConversion is a conversion into an enum, outside its package,
	// of anything but a member's value or zero.
	RuleForgedConversion
	// RuleNumericVerb is an enum value formatted under a verb that bypasses
	// its String.
	RuleNumericVerb
	// RuleStringBypass is an enum value formatted for its name where fmt calls
	// no String: beneath an unexported field, or as the element of a uint8
	// slice under %s or %q.
	RuleStringBypass
	// RuleLaunderedConversion is an enum value converted to another type
	// outside its package, which keeps its ordinal and drops its type.
	RuleLaunderedConversion
	// RuleOrderedComparison is an ordering of an enum outside its package
	// that ordered-enums does not list.
	RuleOrderedComparison
)

func (r Rule) String() string {
	switch r {
	case RuleStringEnum:
		return "string-enum"
	case RuleUnnamed:
		return "unnamed"
	case RuleUnreadableName:
		return "unreadable-name"
	case RuleAmbiguousName:
		return "ambiguous-name"
	case RuleZeroMember:
		return "zero-member"
	case RuleRestated:
		return "restated"
	case RuleTokenDecision:
		return "token-decision"
	case RuleForgedConversion:
		return "forged-conversion"
	case RuleNumericVerb:
		return "numeric-verb"
	case RuleStringBypass:
		return "string-bypass"
	case RuleLaunderedConversion:
		return "laundered-conversion"
	case RuleOrderedComparison:
		return "ordered-comparison"
	default:
		return fmt.Sprintf("Rule(%d)", int(r))
	}
}
