package ztest

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/zcayou/go-tools/golangci/ztest/internal/filepattern"
	"github.com/zcayou/go-tools/golangci/ztest/layout"
)

// ConfigError is a configuration the linter rejects: settings that do not
// decode, that no test file could satisfy, or that select rules in a way
// that means nothing, all of which [New] rejects; and an explicit api setting
// the module answers with nothing, which the run rejects.
type ConfigError struct {
	// Kind is the category block the problem sits in, or zero for one outside
	// any block.
	Kind layout.Kind

	// Setting is the key at fault, or zero for a problem no single key holds.
	Setting Setting

	// Problem is what is wrong.
	Problem Problem

	// Pattern is the configured pattern at fault, or empty when the problem
	// is with no one pattern.
	Pattern string

	// cause is what decoding, compiling a pattern, the rule selection,
	// or the module said.
	cause error
}

func (e *ConfigError) Error() string {
	return Name + ": " + e.detail()
}

func (e *ConfigError) detail() string {
	switch e.Problem {
	case ProblemUndecodable:
		// register.DecodeSettings already names the step.
		if e.cause == nil {
			return "settings do not decode"
		}
		return e.cause.Error()
	case ProblemMalformedPattern:
		if e.cause == nil {
			return e.key() + ": a pattern no file name could match"
		}
		return e.key() + ": " + e.cause.Error()
	case ProblemNoKindAllowed:
		return "no kind of test file is allowed, so none could exist"
	case ProblemNoAdapterName:
		return "adapter-patterns is empty, so no file could hold the Ginkgo adapter"
	case ProblemNoName:
		return e.Kind.String() + " is allowed but no pattern permits any name"
	case ProblemPatternNamesSource:
		return fmt.Sprintf("%s: pattern %q: %s names a source file, and no file of this kind is named for one",
			e.key(), e.Pattern, filepattern.SourceToken)
	case ProblemPatternNamesNoSource:
		return fmt.Sprintf("%s: pattern %q holds no %s, so it names no source file a test file could pin "+
			"the rendering of", e.key(), e.Pattern, filepattern.SourceToken)
	case ProblemEmptyAPI:
		return e.key() + ": an empty list names no package; leave the key out to judge every API package"
	case ProblemNoModuleRoot:
		if e.cause == nil {
			return e.key() + ": resolving its patterns needs the module root"
		}
		return e.key() + ": resolving its patterns needs the module root: " + e.cause.Error()
	case ProblemUnresolvedAPIPattern:
		// The module loader renders the pattern itself.
		if e.cause == nil {
			return fmt.Sprintf("%s: pattern %q matches no API package", e.key(), e.Pattern)
		}
		return e.key() + ": " + e.cause.Error()
	case ProblemRuleSelection:
		// The selection names its own key.
		if e.cause == nil {
			return "the rule selection means nothing"
		}
		return e.cause.Error()
	case ProblemServesNoRule:
		return e.key() + " configures only " + ruleList(e.serves()) + ", which the rule selection leaves off"
	default:
		return e.key() + ": " + e.Problem.String()
	}
}

// key is the setting at fault as the configuration file spells it.
func (e *ConfigError) key() string {
	switch {
	case e.Kind == 0:
		return e.Setting.String()
	case e.Setting == 0:
		return e.Kind.String()
	default:
		return e.Kind.String() + "." + e.Setting.String()
	}
}

// ruleList joins rule tokens for a message, naming a whole half by what
// it judges rather than rule by rule.
func ruleList(rules []fmt.Stringer) string {
	if slices.Equal(rules, layoutRules) {
		return "the layout rules"
	}
	names := make([]string, len(rules))
	for i, rule := range rules {
		names[i] = rule.String()
	}
	return strings.Join(names, " and ")
}

// Unwrap returns what decoding, compiling a pattern, the rule selection,
// or the module said.
func (e *ConfigError) Unwrap() error {
	return e.cause
}

// Setting is a key of [Settings] or of a [Category] block.
type Setting int

const (
	// SettingPatterns is a category's patterns.
	SettingPatterns Setting = iota + 1
	// SettingHelperPatterns is a category's helper-patterns.
	SettingHelperPatterns
	// SettingAdapterPatterns is adapter-patterns.
	SettingAdapterPatterns
	// SettingAPI is api.
	SettingAPI
	// SettingRenderingPatterns is rendering-patterns.
	SettingRenderingPatterns
	// SettingBuildTags is build-tags.
	SettingBuildTags
)

// String is the setting's key.
func (s Setting) String() string {
	switch s {
	case SettingPatterns:
		return "patterns"
	case SettingHelperPatterns:
		return "helper-patterns"
	case SettingAdapterPatterns:
		return "adapter-patterns"
	case SettingAPI:
		return "api"
	case SettingRenderingPatterns:
		return "rendering-patterns"
	case SettingBuildTags:
		return "build-tags"
	default:
		return "Setting(" + strconv.Itoa(int(s)) + ")"
	}
}

// Problem is why a configuration is rejected.
type Problem int

const (
	// ProblemUndecodable is settings that do not decode, such as an unknown key.
	ProblemUndecodable Problem = iota + 1
	// ProblemMalformedPattern is a pattern no file name could match.
	ProblemMalformedPattern
	// ProblemNoKindAllowed is every kind of test file disallowed.
	ProblemNoKindAllowed
	// ProblemNoAdapterName is an empty adapter-patterns, leaving the Ginkgo
	// adapter nowhere to live.
	ProblemNoAdapterName
	// ProblemNoName is an allowed kind no pattern gives a name.
	ProblemNoName
	// ProblemPatternNamesSource is a standalone pattern naming a source file,
	// which a standalone file sits beside none of.
	ProblemPatternNamesSource
	// ProblemPatternNamesNoSource is a rendering pattern holding no <source>,
	// so it names no source whose rendering a test file could pin.
	ProblemPatternNamesNoSource
	// ProblemEmptyAPI is an explicitly empty api, which names no package.
	ProblemEmptyAPI
	// ProblemNoModuleRoot is an explicit api with no module root to resolve
	// it from.
	ProblemNoModuleRoot
	// ProblemUnresolvedAPIPattern is an api pattern the module answers with no API
	// package.
	ProblemUnresolvedAPIPattern
	// ProblemRuleSelection is a rule selection that means nothing, which
	// the cause says how, naming the key.
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
	case ProblemMalformedPattern:
		return "malformed-pattern"
	case ProblemNoKindAllowed:
		return "no-kind-allowed"
	case ProblemNoAdapterName:
		return "no-adapter-name"
	case ProblemNoName:
		return "no-name"
	case ProblemPatternNamesSource:
		return "pattern-names-source"
	case ProblemPatternNamesNoSource:
		return "pattern-names-no-source"
	case ProblemEmptyAPI:
		return "empty-api"
	case ProblemNoModuleRoot:
		return "no-module-root"
	case ProblemUnresolvedAPIPattern:
		return "unresolved-api-pattern"
	case ProblemRuleSelection:
		return "rule-selection"
	case ProblemServesNoRule:
		return "serves-no-rule"
	default:
		return "Problem(" + strconv.Itoa(int(p)) + ")"
	}
}
