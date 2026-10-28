package engine

import "fmt"

// Setting is a configuration key: one an exception entry is listed under,
// or one that widens what the rules judge.
type Setting int

const (
	// SettingOpenEnums lists types whose values come from outside the module.
	SettingOpenEnums Setting = iota + 1
	// SettingZeroDefaults lists enums whose zero member is the documented
	// default.
	SettingZeroDefaults
	// SettingRelatedEnums lists pairs whose tokens overlap because they classify
	// different things.
	SettingRelatedEnums
	// SettingOrderedEnums lists enums whose ordinals are an order.
	SettingOrderedEnums
	// SettingTests says whether the use-site rules judge test files.
	SettingTests
	// SettingOrderedInstantiations says whether ordered-comparison judges
	// an instantiation whose type parameter orders the enum.
	SettingOrderedInstantiations
)

func (s Setting) String() string {
	switch s {
	case SettingOpenEnums:
		return "open-enums"
	case SettingZeroDefaults:
		return "zero-defaults"
	case SettingRelatedEnums:
		return "related-enums"
	case SettingOrderedEnums:
		return "ordered-enums"
	case SettingTests:
		return "tests"
	case SettingOrderedInstantiations:
		return "ordered-instantiations"
	default:
		return fmt.Sprintf("Setting(%d)", int(s))
	}
}

// Fault is why an exception entry suppresses nothing.
type Fault int

const (
	// FaultNotEnum is an entry naming a type that is no enum of the module:
	// undeclared, not an integer or string type, declaring no member, or already
	// taken out by open-enums.
	FaultNotEnum Fault = iota + 1
	// FaultNoZeroMember is a zero-defaults entry for an enum with no member
	// at zero.
	FaultNoZeroMember
	// FaultNotRestated is a related-enums pair whose tokens do not overlap the way
	// restated reports.
	FaultNotRestated
	// FaultNotOrdered is an ordered-enums entry for an enum no judged ordering
	// needs it for: nothing outside its package orders it, and nothing inside
	// bounds it by its highest member.
	FaultNotOrdered
)

func (f Fault) String() string {
	switch f {
	case FaultNotEnum:
		return "not-a-enum"
	case FaultNoZeroMember:
		return "no-zero-member"
	case FaultNotRestated:
		return "not-restated"
	case FaultNotOrdered:
		return "not-ordered"
	default:
		return fmt.Sprintf("Fault(%d)", int(f))
	}
}

// ExceptionError is an exception entry that suppresses nothing. An entry
// that matches nothing does not mean what it says, so it fails the run rather
// than lingering after the code it excused has changed.
type ExceptionError struct {
	// Setting is the key the entry is listed under.
	Setting Setting

	// Fault is why the entry suppresses nothing.
	Fault Fault

	// Type is the type the entry names, as configured. For a related pair
	// that is no restatement it is the pair's first type, and for a pair
	// naming a type that is no enum it is that type.
	Type string

	// Related is a related pair's other type, as configured. It is empty
	// for the other settings.
	Related string
}

func (e *ExceptionError) Error() string {
	switch e.Fault {
	case FaultNotEnum:
		return fmt.Sprintf("%s: %s is not an enum of this module", e.Setting, e.Type)
	case FaultNoZeroMember:
		return fmt.Sprintf("%s: %s has no member at zero", e.Setting, e.Type)
	case FaultNotRestated:
		return fmt.Sprintf("%s: %s and %s do not restate each other's tokens", e.Setting, e.Type, e.Related)
	case FaultNotOrdered:
		return fmt.Sprintf("%s: nothing outside %s's package orders it, "+
			"and nothing inside bounds it by its highest member", e.Setting, e.Type)
	default:
		return fmt.Sprintf("%s: %s suppresses nothing", e.Setting, e.Type)
	}
}
