package engine

import "strconv"

// Rule is a rule deadcode runs beside its verdicts. A verdict
// is the reachability analysis itself and is not selectable; a rule judges
// something further, and a configuration chooses whether it runs.
type Rule int

const (
	// RuleInertField is an exported field of a struct the module takes from its
	// caller that no live module code reads.
	RuleInertField Rule = iota + 1
	// RuleWriteOnlyField is a field of a named struct type that module code
	// writes and no code of its audience reads.
	RuleWriteOnlyField
)

// String is the rule's token, which opens its findings' messages and which
// a rule selection names it by.
func (r Rule) String() string {
	switch r {
	case RuleInertField:
		return "inert-field"
	case RuleWriteOnlyField:
		return "write-only-field"
	default:
		return "Rule(" + strconv.Itoa(int(r)) + ")"
	}
}
