package engine

import "strconv"

// Rule is the rule a finding comes from. Its String is the token that opens
// the finding's message, which is what a linters.exclusions text rule matches.
type Rule int

// The rules, as the package documentation defines them.
const (
	// RuleUnstructuredReturn is a return in a judged function that can carry
	// an error its caller cannot tell apart from another.
	RuleUnstructuredReturn Rule = iota + 1

	// RuleTextDecision is a branch taken on an error's text.
	RuleTextDecision

	// RuleLiteralField is a string field of an error type that only ever holds
	// constants.
	RuleLiteralField

	// RuleMissingBase is an error type that does not carry the configured base.
	RuleMissingBase
)

func (r Rule) String() string {
	switch r {
	case RuleUnstructuredReturn:
		return "unstructured-return"
	case RuleTextDecision:
		return "text-decision"
	case RuleLiteralField:
		return "literal-field"
	case RuleMissingBase:
		return "missing-base"
	default:
		return "Rule(" + strconv.Itoa(int(r)) + ")"
	}
}
