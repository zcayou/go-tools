package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// ConfigError is a configuration deadcode rejects: settings a driver could
// not decode into a [Config], a setting without what it requires, a value
// outside its enum, or a pattern or root program that does not resolve
// to what it claims. The deadcode plugin rejects settings with it before any
// analysis runs, and [Analyze] with it once the module answers.
type ConfigError struct {
	// Setting is the key at fault, as the deadcode plugin spells it.
	Setting Setting

	// Problem is what is wrong with it.
	Problem Problem

	// Value is the configured entry at fault: a pattern, an exemption, a rooting,
	// a root program's file, or the directory a root program would be synthesized
	// in.
	Value string

	// Package is what a root program declares instead of main,
	// for [ProblemNotMain].
	Package string

	// Packages are the packages declared under both api and test-facing,
	// for [ProblemOverlapsAPI].
	Packages []string

	// Rule is the rule at fault, for [ProblemRequiresAPI] when a rule selection
	// enables a rule that requires api.
	Rule Rule

	// cause is what the decoder, the loader, the file system, or the parser
	// said.
	cause error
}

// Undecodable returns the [ConfigError] for settings a driver could not decode
// into a [Config], carrying what its decoder said.
func Undecodable(cause error) *ConfigError {
	return &ConfigError{Problem: ProblemUndecodable, cause: cause}
}

// RuleSelection returns the [ConfigError] for a rule selection that means
// nothing, for the reason cause gives, which names the key.
func RuleSelection(cause error) *ConfigError {
	return &ConfigError{Problem: ProblemRuleSelection, cause: cause}
}

func (e *ConfigError) Error() string {
	switch e.Problem {
	case ProblemUndecodable:
		// A driver's decoder names the step itself.
		if e.cause == nil {
			return "settings do not decode"
		}
		return e.cause.Error()
	case ProblemEmpty:
		if omitted, ok := e.Setting.omitted(); ok {
			return fmt.Sprintf("%s is empty: omit it to %s", e.Setting, omitted)
		}
		return fmt.Sprintf("%s is empty", e.Setting)
	case ProblemUnknownValue:
		if choices, ok := e.Setting.choices(); ok {
			return fmt.Sprintf("unknown %s %q: want one of %s", e.Setting, e.Value, choices)
		}
		return fmt.Sprintf("unknown %s %q", e.Setting, e.Value)
	case ProblemRequiresAPI, ProblemRequiresTests:
		return e.requirement()
	case ProblemUnresolvable:
		if e.cause == nil {
			return fmt.Sprintf("resolving %s pattern %s", e.Setting, e.Value)
		}
		return fmt.Sprintf("resolving %s pattern %s: %v", e.Setting, e.Value, e.cause)
	case ProblemMatchesNothing:
		if e.Setting == SettingRoots {
			return fmt.Sprintf("%s pattern %q matched no files", e.Setting, e.Value)
		}
		return fmt.Sprintf("%s pattern %s matched no packages", e.Setting, e.Value)
	case ProblemOverlapsAPI:
		return "declared both api and test-facing: " + strings.Join(e.Packages, ", ")
	case ProblemReservedDirectory, ProblemUnreadable, ProblemNotMain, ProblemNoMain:
		return e.rootProgram()
	case ProblemRuleSelection, ProblemServesNoRule:
		return e.selection()
	case ProblemUnexplained, ProblemNamesNoFunction, ProblemChangesNothing:
		return e.entry()
	default:
		return fmt.Sprintf("%s: %s", e.Setting, e.Problem)
	}
}

// requirement says what a setting, or a rule a selection enables, is given
// without.
func (e *ConfigError) requirement() string {
	subject := e.Setting.String()
	if e.Rule != 0 {
		subject = e.Rule.String()
	}
	if e.Problem == ProblemRequiresTests {
		return subject + " requires tests: the masked view is all it speaks to"
	}
	return subject + " requires api"
}

// rootProgram says what is wrong with a root program, or with the directory
// they are synthesized in.
func (e *ConfigError) rootProgram() string {
	switch e.Problem {
	case ProblemReservedDirectory:
		return fmt.Sprintf("%s exists in %s: the directory is reserved for synthesized root programs", rootsDir, e.Value)
	case ProblemUnreadable:
		if e.cause == nil {
			return "reading root program " + e.Value
		}
		return fmt.Sprintf("reading root program %s: %v", e.Value, e.cause)
	case ProblemNotMain:
		return fmt.Sprintf("root program %s declares package %s, not main", e.Value, e.Package)
	default:
		return fmt.Sprintf("root program %s declares no func main", e.Value)
	}
}

// selection says what is wrong with the rule selection.
func (e *ConfigError) selection() string {
	switch {
	case e.Problem == ProblemServesNoRule:
		rules := make([]string, 0, len(e.Setting.Serves()))
		for _, rule := range e.Setting.Serves() {
			rules = append(rules, rule.String())
		}
		return fmt.Sprintf("%s configures only %s, which the rule selection leaves off",
			e.Setting, strings.Join(rules, " and "))
	case e.cause == nil:
		// The selection names its own key.
		return "the rule selection means nothing"
	default:
		return e.cause.Error()
	}
}

// entry says what is wrong with an echo-functions entry.
func (e *ConfigError) entry() string {
	switch e.Problem {
	case ProblemUnexplained:
		return fmt.Sprintf("%s entry %s carries no reason", e.Setting, e.Value)
	case ProblemNamesNoFunction:
		return fmt.Sprintf("%s entry %q names no function of the module", e.Setting, e.Value)
	default:
		return fmt.Sprintf("%s entry %s changes no verdict", e.Setting, e.Value)
	}
}

// Unwrap returns what the decoder, the loader, the file system, or the parser
// said.
func (e *ConfigError) Unwrap() error {
	return e.cause
}

// Setting is a key of the deadcode plugin's settings, which [Config] mirrors.
type Setting int

const (
	// SettingPatterns is patterns, [Config.Patterns].
	SettingPatterns Setting = iota + 1
	// SettingAPI is api, [Config.API].
	SettingAPI
	// SettingAPIExempt is api-exempt, [Config.APIExempt].
	SettingAPIExempt
	// SettingAPIGenerics is api-generics, [Config.APIGenerics].
	SettingAPIGenerics
	// SettingAPITestConsumers is api-test-consumers, [Config.APITestConsumers].
	SettingAPITestConsumers
	// SettingTestFacing is test-facing, [Config.TestFacing].
	SettingTestFacing
	// SettingRoots is roots, [Config.Roots].
	SettingRoots
	// SettingEchoFunctions is echo-functions, [Config.EchoFunctions].
	SettingEchoFunctions
)

// String is the setting's key.
func (s Setting) String() string {
	switch s {
	case SettingPatterns:
		return "patterns"
	case SettingAPI:
		return "api"
	case SettingAPIExempt:
		return "api-exempt"
	case SettingAPIGenerics:
		return "api-generics"
	case SettingAPITestConsumers:
		return "api-test-consumers"
	case SettingTestFacing:
		return "test-facing"
	case SettingRoots:
		return "roots"
	case SettingEchoFunctions:
		return "echo-functions"
	default:
		return "Setting(" + strconv.Itoa(int(s)) + ")"
	}
}

// Serves returns the rules beside the verdicts a setting configures, or nil
// for a setting that configures the verdicts.
func (s Setting) Serves() []Rule {
	if s == SettingEchoFunctions {
		return []Rule{RuleInertField}
	}
	return nil
}

// omitted says what leaving the setting out asks for, for [ProblemEmpty],
// or reports false for a setting that is not a list.
func (s Setting) omitted() (string, bool) {
	switch s {
	case SettingPatterns:
		return "analyze the whole module", true
	case SettingTestFacing:
		return "declare no test-facing packages", true
	case SettingRoots:
		return "declare no root programs", true
	case SettingEchoFunctions:
		return "declare no echo functions", true
	default:
		return "", false
	}
}

// choices lists the values the setting takes, for [ProblemUnknownValue],
// or reports false for a setting that takes no closed set of values.
func (s Setting) choices() (string, bool) {
	var names []string
	switch s {
	case SettingAPIExempt:
		for _, kind := range Kinds() {
			names = append(names, kind.String())
		}
	case SettingAPIGenerics:
		for _, rooting := range GenericRootings() {
			names = append(names, rooting.String())
		}
	default:
		return "", false
	}
	return strings.Join(names, ", "), true
}

// Problem is why a configuration is rejected.
type Problem int

const (
	// ProblemUndecodable is settings a driver could not decode into a [Config],
	// such as ones holding an unknown key. [Undecodable] makes one.
	ProblemUndecodable Problem = iota + 1
	// ProblemEmpty is a list written out empty where an absent key is what
	// asks for nothing.
	ProblemEmpty
	// ProblemUnknownValue is a value outside the setting's enum.
	ProblemUnknownValue
	// ProblemRequiresAPI is a setting that speaks only to a declared API surface,
	// given without one.
	ProblemRequiresAPI
	// ProblemRequiresTests is a setting that speaks only to the masked view,
	// given without tests.
	ProblemRequiresTests
	// ProblemUnresolvable is a pattern the loader or the file system could not
	// resolve.
	ProblemUnresolvable
	// ProblemMatchesNothing is a pattern resolving to nothing.
	ProblemMatchesNothing
	// ProblemOverlapsAPI is a package declared both api and test-facing.
	ProblemOverlapsAPI
	// ProblemReservedDirectory is a module already holding the directory root
	// programs are synthesized in.
	ProblemReservedDirectory
	// ProblemUnreadable is a root program that could not be read or parsed.
	ProblemUnreadable
	// ProblemNotMain is a root program declaring a package other than main.
	ProblemNotMain
	// ProblemNoMain is a root program declaring no func main.
	ProblemNoMain
	// ProblemRuleSelection is a rule selection that means nothing, which
	// the cause says how, naming the key.
	ProblemRuleSelection
	// ProblemServesNoRule is a setting given while every rule it configures
	// is off, so it configures nothing.
	ProblemServesNoRule
	// ProblemUnexplained is an entry carrying no reason.
	ProblemUnexplained
	// ProblemNamesNoFunction is an echo-functions entry naming no function
	// the module declares.
	ProblemNamesNoFunction
	// ProblemChangesNothing is an echo-functions entry whose exemption changes
	// no verdict: nothing it reads would otherwise keep a field from being
	// reported.
	ProblemChangesNothing
)

// String is the problem's token.
func (p Problem) String() string {
	switch p {
	case ProblemUndecodable:
		return "undecodable"
	case ProblemEmpty:
		return "empty"
	case ProblemUnknownValue:
		return "unknown-value"
	case ProblemRequiresAPI:
		return "requires-api"
	case ProblemRequiresTests:
		return "requires-tests"
	case ProblemUnresolvable:
		return "unresolvable"
	case ProblemMatchesNothing:
		return "matches-nothing"
	case ProblemOverlapsAPI:
		return "overlaps-api"
	case ProblemReservedDirectory:
		return "reserved-directory"
	case ProblemUnreadable:
		return "unreadable"
	case ProblemNotMain:
		return "not-main"
	case ProblemNoMain:
		return "no-main"
	case ProblemRuleSelection:
		return "rule-selection"
	case ProblemServesNoRule:
		return "serves-no-rule"
	case ProblemUnexplained:
		return "unexplained"
	case ProblemNamesNoFunction:
		return "names-no-function"
	case ProblemChangesNothing:
		return "changes-nothing"
	default:
		return "Problem(" + strconv.Itoa(int(p)) + ")"
	}
}

// RunError is an analysis that could not finish: the program would not load
// or build, or the caller canceled it.
type RunError struct {
	// Stage is where the analysis stopped.
	Stage Stage

	// cause is what stopped it.
	cause error
}

func (e *RunError) Error() string {
	if e.cause == nil {
		return e.Stage.String() + ": stopped"
	}
	return e.Stage.String() + ": " + e.cause.Error()
}

// Unwrap returns what stopped the analysis: the loader's error,
// or the context's.
func (e *RunError) Unwrap() error {
	return e.cause
}

// Stage is a step of the analysis, between which cancellation is observed.
type Stage int

const (
	// StageLoadingPackages loads and type checks the program.
	StageLoadingPackages Stage = iota + 1
	// StageBuildingSSA builds and completes the SSA program.
	StageBuildingSSA
	// StageScanningBinds scans the program for interface binds.
	StageScanningBinds
	// StageAnalyzingReachability runs Rapid Type Analysis from the roots.
	StageAnalyzingReachability
)

// String names the stage.
func (s Stage) String() string {
	switch s {
	case StageLoadingPackages:
		return "loading packages"
	case StageBuildingSSA:
		return "building ssa"
	case StageScanningBinds:
		return "scanning interface binds"
	case StageAnalyzingReachability:
		return "analyzing reachability"
	default:
		return "Stage(" + strconv.Itoa(int(s)) + ")"
	}
}
