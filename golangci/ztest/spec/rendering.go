package spec

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/zcayou/go-tools/golangci/ztest/internal/filepattern"
)

// directory is the on-disk view of the directory a pass's test files sit in:
// the source files a rendering pattern's <source> resolves against. It is read
// from disk rather than taken from the pass because the external test package's
// pass holds none of the sources, and it is read only once a test file's name
// matches a rendering pattern.
type directory struct {
	path    string
	sources filepattern.Sources
	read    bool

	// tests are the names of the directory's test files, in name order.
	tests []string

	// candidates are what has been read of each test file a rendering pattern
	// names for a source declaring an error type, by name.
	candidates map[string]candidate
}

func (d *directory) load() error {
	if d.read {
		return nil
	}
	entries, err := os.ReadDir(d.path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case entry.IsDir() || !strings.HasSuffix(name, ".go"):
		case strings.HasSuffix(name, testSuffix):
			d.tests = append(d.tests, name)
		default:
			d.sources = append(d.sources, name)
		}
	}
	d.read = true
	return nil
}

// renderedSources returns the paths of the sources beside the test file named
// name that a rendering pattern names it for, under any of their
// build-constrained variants.
func (c *checker) renderedSources(name string, dir *directory) ([]string, error) {
	var sources []string
	for _, p := range c.config.Renderings {
		base, ok := p.Source(name)
		if !ok {
			continue
		}
		if err := dir.load(); err != nil {
			return nil, err
		}
		for _, source := range dir.sources.Files(base) {
			sources = append(sources, filepath.Join(dir.path, source))
		}
	}
	return sources, nil
}

// pinsText reports whether a test file named for sources pins an error's text:
// one of them declares a method Error() string.
func pinsText(sources []string) (bool, error) {
	for _, source := range sources {
		declares, err := declaresError(source)
		if err != nil || declares {
			return declares, err
		}
	}
	return false, nil
}

// declaresError reports whether the source file at path declares a method
// Error() string. A file that will not parse could declare anything,
// and the driver reports it against its own package, so it is taken to declare
// one and nothing is said about the test files named for it.
func declaresError(path string) (bool, error) {
	file, err := parseSource(path)
	switch {
	case errors.Is(err, errUnparsed):
		return true, nil
	case err != nil:
		return false, err
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && isErrorMethod(fn) {
			return true, nil
		}
	}
	return false, nil
}

// errUnparsed marks a source file that will not parse.
var errUnparsed = errors.New("file does not parse")

// parseSource parses a source file the pass may not hold. Reading goes through
// the filesystem rather than [analysis.Pass.ReadFile], which serves only
// the files of its own pass.
//
// [analysis.Pass.ReadFile]: https://pkg.go.dev/golang.org/x/tools/go/analysis#Pass.ReadFile
func parseSource(path string) (*ast.File, error) {
	src, err := os.ReadFile(path) //nolint:gosec // the path is a source file beside the test files the pass holds
	if err != nil {
		return nil, err
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, errUnparsed
	}
	return file, nil
}

func isErrorMethod(fn *ast.FuncDecl) bool {
	if fn.Recv == nil || fn.Name.Name != "Error" || fn.Type.Params.NumFields() != 0 ||
		fn.Type.Results.NumFields() != 1 {
		return false
	}
	result, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
	return ok && result.Name == "string"
}
