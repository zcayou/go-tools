package wholemodule

import (
	"fmt"
	"os"
	"path/filepath"
)

// ModuleRoot walks up from the working directory to the nearest go.mod, so
// an analysis covers the whole module however deep golangci-lint was invoked.
// A subdirectory would otherwise miss the importers whose code the analysis
// judges. It returns a [*NoModuleError] when no go.mod is found.
func ModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	start := dir
	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", &NoModuleError{Dir: start}
		}
		dir = parent
	}
}

// NoModuleError is a working directory with no go.mod at or above it.
type NoModuleError struct {
	// Dir is the working directory the walk started from.
	Dir string
}

func (e *NoModuleError) Error() string {
	if e.Dir == "" {
		return "no go.mod at or above the working directory"
	}
	return "no go.mod at or above " + e.Dir
}
