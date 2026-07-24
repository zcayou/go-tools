package testlayout

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// directory is the on-disk view of one package directory: the source files
// a [sourceToken] pattern resolves against, and the name of every test file
// sitting in it.
//
// It is read from disk rather than taken from the pass because a pass holds one
// test package. A directory's internal and external test packages are separate
// passes, while the Ginkgo adapter is one per test binary, so deciding whether
// an adapter exists at all means looking wider than a pass can see.
type directory struct {
	path    string
	sources sources

	// tests are the test file names, in the order [os.ReadDir] returns them, which
	// is sorted. That is what makes the file a missing adapter is reported against
	// the same one for either pass covering the directory. A name here is a name
	// on disk and nothing more: whether a build compiles the file is settled
	// by whoever reads it.
	tests []string
}

func readDirectory(path string) (directory, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return directory{}, fmt.Errorf("reading %s: %w", path, err)
	}

	dir := directory{path: path}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, testSuffix) {
			dir.tests = append(dir.tests, name)
		} else {
			dir.sources = append(dir.sources, name)
		}
	}
	return dir, nil
}

// sources are the names of the files in one directory that are not test files.
type sources []string

// declares reports whether a source file named base sits in the directory.
// A build-constrained variant counts: platform_linux.go is the source
// platform_test.go exercises, so a split source is not read as a missing one.
func (s sources) declares(base string) bool {
	return slices.ContainsFunc(s, func(name string) bool {
		return name == base+".go" || isBuildVariant(name, base)
	})
}

// isBuildVariant reports whether name is base under a GOOS or GOARCH file-name
// constraint, the implicit build tag go/build reads out of a name.
func isBuildVariant(name, base string) bool {
	rest, ok := strings.CutPrefix(name, base+"_")
	if !ok {
		return false
	}
	if rest, ok = strings.CutSuffix(rest, ".go"); !ok {
		return false
	}
	if goosValues[rest] || goarchValues[rest] {
		return true
	}
	goos, goarch, ok := strings.Cut(rest, "_")
	return ok && goosValues[goos] && goarchValues[goarch]
}

// goosValues and goarchValues are the values go/build recognizes
// in the trailing element of a file name, which is where the constraint a name
// carries comes from.
var goosValues = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true,
	"illumos": true, "ios": true, "js": true, "linux": true, "netbsd": true, "openbsd": true,
	"plan9": true, "solaris": true, "unix": true, "wasip1": true, "windows": true,
}

var goarchValues = map[string]bool{
	"386": true, "amd64": true, "arm": true, "arm64": true, "loong64": true, "mips": true,
	"mips64": true, "mips64le": true, "mipsle": true, "ppc64": true, "ppc64le": true,
	"riscv64": true, "s390x": true, "wasm": true,
}
