package engine

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// testFacing is the declared set of packages whose intended consumers
// are tests. Declaring one extends the masked view's definition of test origin:
// the package is test code the compiler cannot recognize, so under the mask its
// facts are removed exactly as _test.go facts are and it contributes no roots,
// while the full view — and so full-view output — is untouched.
type testFacing struct {
	patterns []string
	packages map[string]bool
}

// newTestFacing validates the declared patterns. Declaring them without tests
// is rejected rather than ignored: the masked view is all the declaration
// speaks to, and without tests there is none.
func newTestFacing(patterns []string, tests bool) (*testFacing, error) {
	declared := &testFacing{}
	for _, pattern := range patterns {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			declared.patterns = append(declared.patterns, pattern)
		}
	}
	if len(declared.patterns) > 0 && !tests {
		return nil, errors.New("test-facing requires tests")
	}
	return declared, nil
}

// resolve turns the patterns into the package paths they match and checks them
// against the resolved API surface. A package declared under both is refused:
// the two assert contradictory facts about who its consumers are, and rooting
// it as api in the masked view would resurrect everything it reaches — exactly
// the blind spot the test-only family exists to close.
func (t *testFacing) resolve(load *packages.Config, surface *apiSurface) error {
	if len(t.patterns) == 0 {
		return nil
	}
	resolved, err := resolvePatterns(load, "test-facing", t.patterns)
	if err != nil {
		return err
	}
	var overlap []string
	for path := range resolved {
		if surface.packages[path] {
			overlap = append(overlap, path)
		}
	}
	if len(overlap) > 0 {
		slices.Sort(overlap)
		return fmt.Errorf("declared both api and test-facing: %s", strings.Join(overlap, ", "))
	}
	t.packages = resolved
	return nil
}
