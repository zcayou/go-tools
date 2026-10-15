package layout

import (
	"slices"
	"strconv"

	"github.com/zcayou/go-tools/golangci/ztest/internal/filepattern"
)

// Config is what the layout rules judge test files against: each kind of test
// file and the names it may have, where the Ginkgo adapter may live, and which
// rules run. The ztest linter resolves it from its settings.
type Config struct {
	// Whitebox, Blackbox and Standalone are the kinds of test file.
	Whitebox   Category
	Blackbox   Category
	Standalone Category

	// Adapters are the names a file holding the Ginkgo adapter may have.
	Adapters filepattern.Set

	// Disabled are rules that report nothing. Every rule runs by default.
	Disabled []Rule
}

// Category is one kind of test file: whether files of that kind may exist,
// and what they may be named.
type Category struct {
	// Kind is the kind, and the settings block that configures it, so a diagnostic
	// points at the knob that decides it.
	Kind Kind

	// Allowed reports whether a test file of this kind may exist at all.
	Allowed bool

	// Sourceless marks the kind whose files sit beside no source: no pattern
	// of theirs can resolve [filepattern.SourceToken], and a file of the kind
	// that may not exist has no package rename to be offered.
	Sourceless bool

	// Specs are the names a spec-bearing file of this kind may have.
	Specs filepattern.Set

	// Helpers are the names a file of this kind carrying no specs may have.
	Helpers filepattern.Set
}

// Kind is a kind of test file, and the settings block configuring it.
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

// category picks the kind a test file is: standalone when its directory holds
// no source to test, and otherwise whichever package the file declares.
func (c Config) category(file testFile, dir directory) Category {
	switch {
	case dir.standalone:
		return c.Standalone
	case file.blackbox:
		return c.Blackbox
	default:
		return c.Whitebox
	}
}

// forbidden says what to do about a file of this kind, which may not exist.
// Beside a source the answer is the other package; a standalone file has
// no other package to declare, so the answer is to move beside a source.
func (c Category) forbidden(pkg string) string {
	if c.Sourceless {
		return "a test file sits beside the source it exercises"
	}
	return "declare package " + otherPackage(pkg)
}

// names lists every name a file of this kind may have.
func (c Category) names() string {
	return slices.Concat(c.Specs, c.Helpers).String()
}

// specsBelong says where the specs a helper file holds should have gone.
// An explicitly empty list of spec patterns leaves nowhere to send them, so
// the setting that decided it is named instead.
func (c Category) specsBelong() string {
	if len(c.Specs) == 0 {
		return c.Kind.String() + ".patterns is empty, so no file of this kind may carry specs"
	}
	return "move them to a file named " + c.Specs.String()
}

// helpersBelong says what a file named for a source but carrying no specs
// should have been called. An explicitly empty list of helper patterns leaves
// no such name, so the setting that decided it is named instead.
func (c Category) helpersBelong() string {
	if len(c.Helpers) == 0 {
		return c.Kind.String() + ".helper-patterns is empty, so every file of this kind must carry specs"
	}
	return "a file carrying only helpers must be named " + c.Helpers.String()
}

// fixturesBelong says where a fixture another file consumes should have been
// declared. An explicitly empty list of helper patterns leaves no such file,
// so the setting that decided it is named instead.
func (c Category) fixturesBelong() string {
	if len(c.Helpers) == 0 {
		return c.Kind.String() + ".helper-patterns is empty, so no file of this kind may hold a fixture another file uses"
	}
	return "a fixture another file uses belongs in a file named " + c.Helpers.String()
}
