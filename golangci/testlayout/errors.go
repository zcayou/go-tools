package testlayout

import (
	"fmt"
	"strconv"

	"github.com/zcayou/go-tools/golangci/internal/filepattern"
)

// ConfigError is a configuration the linter rejects: settings that do not
// decode, or that no test file could satisfy.
type ConfigError struct {
	// Kind is the category block the problem sits in, or zero for one outside
	// any block.
	Kind Kind

	// Setting is the key at fault, or zero for a problem no single key holds.
	Setting Setting

	// Problem is what is wrong.
	Problem Problem

	// Pattern is the configured pattern at fault, for [ProblemSourcelessPattern].
	Pattern string

	// cause is what decoding, or compiling a pattern, said.
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
	case ProblemSourcelessPattern:
		return fmt.Sprintf("%s: pattern %q: %s names a source file, and no file of this kind is named for one",
			e.key(), e.Pattern, filepattern.SourceToken)
	default:
		return e.key() + ": " + e.Problem.String()
	}
}

// key is the setting at fault as the configuration file spells it.
func (e *ConfigError) key() string {
	if e.Kind == 0 {
		return e.Setting.String()
	}
	return e.Kind.String() + "." + e.Setting.String()
}

// Unwrap returns what decoding, or compiling a pattern, said.
func (e *ConfigError) Unwrap() error {
	return e.cause
}

// Kind is a kind of test file, and the category block configuring it.
type Kind int

const (
	// KindWhitebox is a test file declaring the package under test.
	KindWhitebox Kind = iota + 1
	// KindBlackbox is a test file declaring the external <pkg>_test package.
	KindBlackbox
	// KindStandalone is a test file in a directory holding no source to test.
	KindStandalone
)

// String is the kind's settings key.
func (k Kind) String() string {
	switch k {
	case KindWhitebox:
		return "whitebox"
	case KindBlackbox:
		return "blackbox"
	case KindStandalone:
		return "standalone"
	default:
		return "Kind(" + strconv.Itoa(int(k)) + ")"
	}
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
	// ProblemSourcelessPattern is a standalone pattern naming a source file,
	// which a standalone file sits beside none of.
	ProblemSourcelessPattern
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
	case ProblemSourcelessPattern:
		return "sourceless-pattern"
	default:
		return "Problem(" + strconv.Itoa(int(p)) + ")"
	}
}
