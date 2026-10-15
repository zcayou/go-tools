package engine

import (
	"strconv"
	"strings"
)

// ConfigError is configuration zerrors rejects: settings that do not decode
// or that no module could satisfy, which the plugin rejects before any analysis
// runs, and a [Config] naming, matching or exempting nothing in the module,
// which [Analyze] rejects.
type ConfigError struct {
	// Setting is the key at fault, as the zerrors plugin spells it, or zero
	// when the settings did not decode.
	Setting Setting

	// Problem is what is wrong with it.
	Problem Problem

	// Value is the configured entry at fault.
	Value string

	// cause is what decoding the settings, selecting rules, or resolving
	// an api pattern ran into.
	cause error
}

// Undecodable is settings that do not decode, for the reason cause gives.
func Undecodable(cause error) *ConfigError {
	return &ConfigError{Problem: ProblemUndecodable, cause: cause}
}

// RuleSelection is a rule selection that means nothing, for the reason cause
// gives, which names the key.
func RuleSelection(cause error) *ConfigError {
	return &ConfigError{Problem: ProblemRuleSelection, cause: cause}
}

func (e *ConfigError) Error() string {
	setting := e.Setting.String()
	entry := setting + ": " + strconv.Quote(e.Value)
	switch e.Problem {
	case ProblemUndecodable:
		// The decoder names the step itself, and golangci-lint the plugin.
		if e.cause == nil {
			return "settings do not decode"
		}
		return e.cause.Error()
	case ProblemEmpty:
		switch e.Setting {
		case SettingAPI:
			return "api is empty: omit it to judge every package as API"
		case SettingBasePackages:
			return "base-packages is empty: omit it to judge every package's error types"
		default:
			return setting + " is empty"
		}
	case ProblemWithoutBase:
		return setting + " is set without base, so it scopes nothing"
	case ProblemRuleSelection, ProblemServesNoRule:
		return e.selection()
	case ProblemMalformed:
		return entry + e.malformed()
	case ProblemNoReason:
		return entry + " gives no reason"
	case ProblemMatchesNothing:
		switch {
		case e.cause != nil:
			return setting + ": " + e.cause.Error()
		case e.Setting == SettingTrusted:
			return entry + " matches no package the module imports"
		default:
			return entry + " matches nothing"
		}
	case ProblemUnresolved:
		return entry + e.unresolved()
	case ProblemWrongKind:
		if e.Setting == SettingBase {
			return entry + " names neither a struct nor an interface"
		}
		return entry + " names the wrong kind of declaration"
	case ProblemExemptsNothing:
		if e.Setting == SettingProtocols {
			return entry + " exempts nothing: no method it names is otherwise judged"
		}
		return entry + " exempts nothing"
	default:
		if e.cause != nil {
			return entry + ": " + e.Problem.String() + ": " + e.cause.Error()
		}
		return entry + ": " + e.Problem.String()
	}
}

// selection says what is wrong with the rule selection, or with a setting
// it leaves serving nothing.
func (e *ConfigError) selection() string {
	switch {
	case e.Problem == ProblemServesNoRule:
		return e.Setting.String() + " configures only " + ruleList(e.Setting.Serves()) +
			", which the rule selection leaves off"
	case e.cause == nil:
		// The selection names its own key.
		return "the rule selection means nothing"
	default:
		return e.cause.Error()
	}
}

// malformed says what form the setting's entries take.
func (e *ConfigError) malformed() string {
	switch e.Setting {
	case SettingTrusted:
		return " is not an import path, optionally ending in /..."
	case SettingProtocols:
		return " is not written import/path.Interface.Method"
	case SettingBase:
		return " is not written import/path.Name"
	default:
		return " is malformed"
	}
}

// unresolved says what the setting's entry fails to name.
func (e *ConfigError) unresolved() string {
	switch e.Setting {
	case SettingProtocols:
		return " names no interface method"
	case SettingBase:
		return " names no type"
	default:
		return " names nothing"
	}
}

// Unwrap returns what decoding the settings, selecting rules, or resolving
// an api pattern ran into.
func (e *ConfigError) Unwrap() error {
	return e.cause
}

// Setting is a configuration key an error can name.
type Setting int

const (
	// SettingAPI is api.
	SettingAPI Setting = iota + 1
	// SettingTrusted is trusted.
	SettingTrusted
	// SettingProtocols is protocols.
	SettingProtocols
	// SettingBase is base.
	SettingBase
	// SettingBasePackages is base-packages.
	SettingBasePackages
	// SettingSentinels is sentinels.
	SettingSentinels
)

// String is the setting's key.
func (s Setting) String() string {
	switch s {
	case SettingAPI:
		return "api"
	case SettingTrusted:
		return "trusted"
	case SettingProtocols:
		return "protocols"
	case SettingBase:
		return "base"
	case SettingBasePackages:
		return "base-packages"
	case SettingSentinels:
		return "sentinels"
	default:
		return "Setting(" + strconv.Itoa(int(s)) + ")"
	}
}

// Serves returns the rules a setting configures, or nil for a setting
// that configures whichever run.
func (s Setting) Serves() []Rule {
	switch s {
	case SettingAPI:
		return []Rule{RuleUnstructuredReturn, RuleLiteralField, RuleRenderedField}
	case SettingTrusted, SettingProtocols, SettingSentinels:
		return []Rule{RuleUnstructuredReturn}
	case SettingBase:
		return []Rule{RuleUnstructuredReturn, RuleMissingBase}
	case SettingBasePackages:
		return []Rule{RuleMissingBase}
	default:
		return nil
	}
}

// ruleList joins rule tokens for a message, as a sentence lists them.
func ruleList(rules []Rule) string {
	names := make([]string, len(rules))
	for i, rule := range rules {
		names[i] = rule.String()
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// Problem is why a configuration is rejected.
type Problem int

const (
	// ProblemUndecodable is settings that do not decode, such as an unknown key.
	ProblemUndecodable Problem = iota + 1
	// ProblemEmpty is a list written out empty where an absent key is what asks
	// for the default.
	ProblemEmpty
	// ProblemMalformed is an entry not written the way its setting reads one.
	ProblemMalformed
	// ProblemNoReason is an exception that does not say why it is one.
	ProblemNoReason
	// ProblemMatchesNothing is an entry matching no package it could mean.
	ProblemMatchesNothing
	// ProblemUnresolved is an entry naming a declaration the module does not
	// reach.
	ProblemUnresolved
	// ProblemWrongKind is a base naming a type that is neither a struct nor
	// an interface.
	ProblemWrongKind
	// ProblemExemptsNothing is a protocols entry that takes no judged method out
	// of judgment.
	ProblemExemptsNothing
	// ProblemWithoutBase is base-packages set while base is not, which scopes
	// a rule that is off.
	ProblemWithoutBase
	// ProblemRuleSelection is a rule selection that means nothing, which
	// the cause says how.
	ProblemRuleSelection
	// ProblemServesNoRule is a setting given while every rule it configures
	// is off, so it configures nothing.
	ProblemServesNoRule
)

// String is the problem's token.
func (p Problem) String() string {
	switch p {
	case ProblemUndecodable:
		return "undecodable"
	case ProblemEmpty:
		return "empty"
	case ProblemMalformed:
		return "malformed"
	case ProblemNoReason:
		return "no-reason"
	case ProblemMatchesNothing:
		return "matches-nothing"
	case ProblemUnresolved:
		return "unresolved"
	case ProblemWrongKind:
		return "wrong-kind"
	case ProblemExemptsNothing:
		return "exempts-nothing"
	case ProblemWithoutBase:
		return "without-base"
	case ProblemRuleSelection:
		return "rule-selection"
	case ProblemServesNoRule:
		return "serves-no-rule"
	default:
		return "Problem(" + strconv.Itoa(int(p)) + ")"
	}
}
