package testlayout

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zcayou/go-tools/golangci/internal/filepattern"
)

// directory is the on-disk view of one package directory: the source files
// a [filepattern.SourceToken] pattern resolves against, whether they leave
// anything to test, and the name of every test file sitting in it.
//
// It is read from disk rather than taken from the pass because a pass holds one
// test package. A directory's internal and external test packages are separate
// passes, while the Ginkgo adapter is one per test binary, so deciding whether
// an adapter exists at all means looking wider than a pass can see.
type directory struct {
	path    string
	sources filepattern.Sources

	// standalone reports that the directory holds no source to test: every file
	// in sources declares nothing, as a doc.go does. A test file there
	// is the standalone kind, whatever package it declares.
	standalone bool

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

	standalone, err := declareNothing(path, dir.sources)
	if err != nil {
		return directory{}, err
	}
	dir.standalone = standalone
	return dir, nil
}

// declareNothing reports whether no file of srcs, in the directory at path,
// holds a declaration, which is what leaves nothing to test. A doc.go carrying
// only a package clause declares nothing. A file carrying only an import does
// not count as nothing: a side-effect import makes the package do something,
// and a package that does something is a package under test. The walk stops
// at the first file that declares, so a directory with real sources costs one
// parse.
func declareNothing(path string, srcs filepattern.Sources) (bool, error) {
	for _, name := range srcs {
		file, err := parsePath(filepath.Join(path, name))
		if err != nil {
			return false, err
		}
		if len(file.Decls) > 0 {
			return false, nil
		}
	}
	return true, nil
}
