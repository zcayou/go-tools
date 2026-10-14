package zlines

import "strconv"

// ConfigError is a configuration the linter rejects: settings that do not
// decode, or that hold a value no limit could mean.
type ConfigError struct {
	// Setting is the key at fault, or zero when the settings did not decode.
	Setting Setting

	// Problem is what is wrong with it.
	Problem Problem

	// Value is the value the setting was given.
	Value int

	// Limit is the comment-length in force, for [ProblemMinimumPastLength].
	Limit int

	// cause is what decoding said.
	cause error
}

func (e *ConfigError) Error() string {
	switch e.Problem {
	case ProblemUndecodable:
		// register.DecodeSettings already names the step.
		if e.cause == nil {
			return Name + ": settings do not decode"
		}
		return Name + ": " + e.cause.Error()
	case ProblemNotPositive:
		return Name + ": " + e.Setting.String() + " must be positive, got " + strconv.Itoa(e.Value)
	case ProblemNegative:
		return Name + ": " + e.Setting.String() + " must not be negative, got " + strconv.Itoa(e.Value)
	case ProblemMinimumPastLength:
		return Name + ": " + e.Setting.String() + " " + strconv.Itoa(e.Value) + " exceeds " +
			SettingCommentLength.String() + " " + strconv.Itoa(e.Limit)
	case ProblemEmptyPrefix:
		return Name + ": " + e.Setting.String() + " must not hold an empty prefix"
	default:
		return Name + ": " + e.Setting.String() + ": " + e.Problem.String()
	}
}

// Unwrap returns what decoding said.
func (e *ConfigError) Unwrap() error {
	return e.cause
}

// Setting is a key of [Settings] a value can be wrong for.
type Setting int

const (
	// SettingLineLength is line-length.
	SettingLineLength Setting = iota + 1
	// SettingCommentLength is comment-length.
	SettingCommentLength
	// SettingCommentMinLength is comment-min-length.
	SettingCommentMinLength
	// SettingCommentOverrun is comment-overrun.
	SettingCommentOverrun
	// SettingCommentExempt is comment-exempt.
	SettingCommentExempt
)

// String is the setting's key.
func (s Setting) String() string {
	switch s {
	case SettingLineLength:
		return "line-length"
	case SettingCommentLength:
		return "comment-length"
	case SettingCommentMinLength:
		return "comment-min-length"
	case SettingCommentOverrun:
		return "comment-overrun"
	case SettingCommentExempt:
		return "comment-exempt"
	default:
		return "Setting(" + strconv.Itoa(int(s)) + ")"
	}
}

// Problem is why a configuration is rejected.
type Problem int

const (
	// ProblemUndecodable is settings that do not decode, such as an unknown key.
	ProblemUndecodable Problem = iota + 1
	// ProblemNotPositive is a length of zero or less, which is a mistake rather
	// than a way to turn a rule off: the rule's own key is for that.
	ProblemNotPositive
	// ProblemNegative is a negative overrun, which would report a lone line
	// the fill itself writes.
	ProblemNegative
	// ProblemMinimumPastLength is a comment minimum past the comment length,
	// a band no paragraph could sit inside.
	ProblemMinimumPastLength
	// ProblemEmptyPrefix is an empty exempt prefix, which every comment opens
	// with, so it would leave comment-wrap enforced but silent.
	ProblemEmptyPrefix
)

// String is the problem's token.
func (p Problem) String() string {
	switch p {
	case ProblemUndecodable:
		return "undecodable"
	case ProblemNotPositive:
		return "not-positive"
	case ProblemNegative:
		return "negative"
	case ProblemMinimumPastLength:
		return "minimum-past-length"
	case ProblemEmptyPrefix:
		return "empty-prefix"
	default:
		return "Problem(" + strconv.Itoa(int(p)) + ")"
	}
}
