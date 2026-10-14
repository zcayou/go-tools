package zassert

import (
	"fmt"
	"strconv"
)

// ConfigError is a configuration zassert cannot run under. [New]
// and [NewAnalyzer] return one for settings no run could satisfy, and a run
// returns one when the module answers an explicit api setting with nothing.
type ConfigError struct {
	// Setting is the key the problem is with. It is zero for settings that do
	// not decode, where no one key is to blame.
	Setting Setting

	// Problem is what is wrong with it.
	Problem Problem

	// Pattern is the configured pattern the problem is with, or empty when
	// it is with no one pattern.
	Pattern string

	// cause is what the decoder, the pattern grammar, or the module said.
	cause error
}

func (e *ConfigError) Error() string {
	switch e.Problem {
	case ProblemUndecodableSettings:
		// The decoder says it was decoding settings.
		if e.cause == nil {
			return Name + ": settings do not decode"
		}
		return fmt.Sprintf("%s: %v", Name, e.cause)
	case ProblemEmptyAPI:
		return fmt.Sprintf("%s: %s: an empty list names no package; leave the key out to judge every API package",
			Name, e.Setting)
	case ProblemSourcelessRenderingPattern:
		return fmt.Sprintf("%s: %s: pattern %q holds no <source>, so it names no source file a test file "+
			"could pin the rendering of", Name, e.Setting, e.Pattern)
	case ProblemNoModuleRoot:
		if e.cause == nil {
			return fmt.Sprintf("%s: %s: resolving its patterns needs the module root", Name, e.Setting)
		}
		return fmt.Sprintf("%s: %s: resolving its patterns needs the module root: %v", Name, e.Setting, e.cause)
	case ProblemMalformedRenderingPattern, ProblemUnresolvedAPIPattern:
		// The pattern grammar and the module loader render the pattern themselves.
		if e.cause == nil {
			return fmt.Sprintf("%s: %s: pattern %q: %s", Name, e.Setting, e.Pattern, e.Problem)
		}
		return fmt.Sprintf("%s: %s: %v", Name, e.Setting, e.cause)
	default:
		return fmt.Sprintf("%s: %s: %s", Name, e.Setting, e.Problem)
	}
}

// Unwrap returns what the decoder, the pattern grammar, or the module said,
// or nil when the problem is the configuration's alone.
func (e *ConfigError) Unwrap() error {
	return e.cause
}

// Setting is a key of zassert's settings block.
type Setting int

const (
	// SettingAPI is the api key, narrowing the packages whose specs
	// unidentified-error judges.
	SettingAPI Setting = iota + 1

	// SettingRenderingPatterns is the rendering-patterns key, naming the test
	// files that pin an error's rendering.
	SettingRenderingPatterns
)

func (s Setting) String() string {
	switch s {
	case SettingAPI:
		return "api"
	case SettingRenderingPatterns:
		return "rendering-patterns"
	default:
		return "Setting(" + strconv.Itoa(int(s)) + ")"
	}
}

// Problem is why zassert cannot run under a configuration.
type Problem int

const (
	// ProblemUndecodableSettings is a settings block that does not decode:
	// an unknown key, or a value of the wrong shape.
	ProblemUndecodableSettings Problem = iota + 1

	// ProblemEmptyAPI is an api setting given as an explicit empty list, which
	// would leave unidentified-error judging nothing.
	ProblemEmptyAPI

	// ProblemMalformedRenderingPattern is a rendering pattern the file-name
	// grammar rejects: one holding a path separator, a malformed glob, a glob
	// wrapped around <source>, or a second <source>.
	ProblemMalformedRenderingPattern

	// ProblemSourcelessRenderingPattern is a rendering pattern holding no
	// <source>, which names no source file whose rendering it could pin.
	ProblemSourcelessRenderingPattern

	// ProblemNoModuleRoot is an explicit api setting where no module root is found
	// to resolve its patterns from.
	ProblemNoModuleRoot

	// ProblemUnresolvedAPIPattern is an api pattern that matches no API package,
	// or that the module loader cannot resolve.
	ProblemUnresolvedAPIPattern
)

func (p Problem) String() string {
	switch p {
	case ProblemUndecodableSettings:
		return "undecodable-settings"
	case ProblemEmptyAPI:
		return "empty-api"
	case ProblemMalformedRenderingPattern:
		return "malformed-rendering-pattern"
	case ProblemSourcelessRenderingPattern:
		return "sourceless-rendering-pattern"
	case ProblemNoModuleRoot:
		return "no-module-root"
	case ProblemUnresolvedAPIPattern:
		return "unresolved-api-pattern"
	default:
		return "Problem(" + strconv.Itoa(int(p)) + ")"
	}
}
