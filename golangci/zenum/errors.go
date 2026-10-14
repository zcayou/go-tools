package zenum

import (
	"fmt"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

// Problem is what is wrong with a settings block or one of its entries.
type Problem int

const (
	// ProblemUndecodableSettings is a settings block that does not decode:
	// an unknown key, or a value of the wrong shape.
	ProblemUndecodableSettings Problem = iota + 1
	// ProblemUnexplainedException is an entry with an empty reason.
	ProblemUnexplainedException
	// ProblemUnqualifiedType is an entry naming a type other than
	// as import/path.Name.
	ProblemUnqualifiedType
)

func (p Problem) String() string {
	switch p {
	case ProblemUndecodableSettings:
		return "undecodable-settings"
	case ProblemUnexplainedException:
		return "unexplained-exception"
	case ProblemUnqualifiedType:
		return "unqualified-type"
	default:
		return fmt.Sprintf("Problem(%d)", int(p))
	}
}

// ConfigError is a settings block [New] rejects.
type ConfigError struct {
	// Problem is what is wrong.
	Problem Problem

	// Setting is the key the rejected entry is listed under. It is zero when
	// the block as a whole did not decode.
	Setting engine.Setting

	// Index is the rejected entry's position in its list.
	Index int

	// Type is the type text the entry gives, when that is what is wrong.
	Type string

	// cause is what decoding said.
	cause error
}

func (e *ConfigError) Error() string {
	switch e.Problem {
	case ProblemUndecodableSettings:
		if e.cause == nil {
			return fmt.Sprintf("%s: settings do not decode", Name)
		}
		// register.DecodeSettings already names the step, and golangci-lint
		// prefixes the plugin.
		return e.cause.Error()
	case ProblemUnexplainedException:
		return fmt.Sprintf("%s: %s[%d]: reason is empty: say why the entry holds", Name, e.Setting, e.Index)
	case ProblemUnqualifiedType:
		return fmt.Sprintf("%s: %s[%d]: %q does not name a type as import/path.Name", Name, e.Setting, e.Index, e.Type)
	default:
		return fmt.Sprintf("%s: %s[%d]: %s", Name, e.Setting, e.Index, e.Problem)
	}
}

// Unwrap returns what decoding said, when the block did not decode.
func (e *ConfigError) Unwrap() error {
	return e.cause
}
