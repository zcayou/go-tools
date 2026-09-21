package testlayout

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ginkgoPackages are the Ginkgo packages a builder, a suite hook, or RunSpecs
// is reached through, each mapped to the name it declares, which is what a file
// refers to it by unless the import renames it. The dsl packages re-export
// slices of the root package, for a file that dot-imports only some of it.
var ginkgoPackages = map[string]string{
	"github.com/onsi/ginkgo/v2":               "ginkgo",
	"github.com/onsi/ginkgo/v2/dsl/core":      "core",
	"github.com/onsi/ginkgo/v2/dsl/reporting": "reporting",
	"github.com/onsi/ginkgo/v2/dsl/table":     "table",
}

// specBuilders are the Ginkgo container and leaf builders. A call to one
// through a Ginkgo import marks the file as spec-bearing and the package
// as using Ginkgo, which is what puts it under the adapter rules.
var specBuilders = map[string]bool{
	"Describe": true, "FDescribe": true, "PDescribe": true, "XDescribe": true,
	"Context": true, "FContext": true, "PContext": true, "XContext": true,
	"When": true, "FWhen": true, "PWhen": true, "XWhen": true,
	"DescribeTable": true, "FDescribeTable": true, "PDescribeTable": true, "XDescribeTable": true,
	"DescribeTableSubtree": true, "FDescribeTableSubtree": true,
	"PDescribeTableSubtree": true, "XDescribeTableSubtree": true,
	"It": true, "FIt": true, "PIt": true, "XIt": true,
	"Specify": true, "FSpecify": true, "PSpecify": true, "XSpecify": true,
}

// testFuncPrefixes are the go test entry-point prefixes. A file declaring one
// carries tests whether or not Ginkgo is involved.
var testFuncPrefixes = []string{"Test", "Benchmark", "Fuzz", "Example"}

// runSpecs is the call that hands a package's registered Ginkgo specs to go
// test. Exactly one file per test binary makes it, and Ginkgo fails the run if
// two do.
const runSpecs = "RunSpecs"

// errUnparsed marks a file the pass does not hold that will not parse. What
// such a file declares is unknown: a test file might hold the adapter,
// and a source file might leave its directory with something to test, so
// nothing is said.
var errUnparsed = errors.New("file does not parse")

// info is what one test file's syntax tells the layout rules.
type info struct {
	// specs reports whether the file carries specs or test functions rather than
	// only helpers and fakes.
	specs bool

	// ginkgo reports whether the file registers Ginkgo specs.
	ginkgo bool

	// adapter is where the file calls [runSpecs], or [token.NoPos]. For a file
	// the pass does not hold it belongs to a throwaway [token.FileSet] and is only
	// ever tested for validity.
	adapter token.Pos
}

func inspect(file *ast.File) info {
	imports := ginkgoImportsOf(file)
	var found info
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			if node.Recv == nil && isTestFuncName(node.Name.Name) {
				found.specs = true
			}
		case *ast.CallExpr:
			switch name := imports.callName(node.Fun); {
			case specBuilders[name]:
				found.specs, found.ginkgo = true, true
			case name == runSpecs && !found.adapter.IsValid():
				found.adapter = node.Pos()
			}
		}
		return true
	})
	return found
}

// inspectPath reads a test file the pass does not hold.
func inspectPath(path string) (info, error) {
	file, err := parsePath(path)
	if err != nil {
		return info{}, err
	}
	return inspect(file), nil
}

// parsePath parses a file the pass does not hold from the directory
// it is analyzing. Reading goes through the filesystem rather than
// [analysis.Pass.ReadFile], which serves only the files of its own pass. A file
// that will not parse yields [errUnparsed]: the driver reports the syntax error
// against the package holding the file, and that package is skipped, so this
// pass is the one left judging a file it cannot read.
//
// [analysis.Pass.ReadFile]: https://pkg.go.dev/golang.org/x/tools/go/analysis#Pass.ReadFile
func parsePath(path string) (*ast.File, error) {
	src, err := os.ReadFile(path) //nolint:gosec // the path is a file in a directory the pass is already analyzing
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, errUnparsed)
	}
	return file, nil
}

func isTestFuncName(name string) bool {
	return slices.ContainsFunc(testFuncPrefixes, func(prefix string) bool {
		return isTestName(name, prefix)
	})
}

// isTestName reports whether name is prefix followed by nothing or by a rune
// that is not lower case, which is the rule go test applies: Test
// and TestWidget name entry points, Testify does not.
func isTestName(name, prefix string) bool {
	rest, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return false
	}
	if rest == "" {
		return true
	}
	first, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(first)
}

// ginkgoImports is how one file refers to Ginkgo. It is read from the file's
// imports rather than from type information, which the syntactic load mode
// does not provide and a file read from disk would not have either.
type ginkgoImports struct {
	// dot reports whether the file dot-imports a Ginkgo package, which is what
	// makes a bare It Ginkgo's.
	dot bool

	// names are the names the file imports a Ginkgo package under, declared
	// or renamed, which is what makes ginkgo.It Ginkgo's.
	names []string
}

func ginkgoImportsOf(file *ast.File) ginkgoImports {
	var imports ginkgoImports
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name, ok := ginkgoPackages[path]
		if !ok {
			continue
		}
		if spec.Name != nil {
			name = spec.Name.Name
		}
		switch name {
		case ".":
			imports.dot = true
		case "_":
			// A blank import brings nothing into the file to call.
		default:
			imports.names = append(imports.names, name)
		}
	}
	return imports
}

// callName is the Ginkgo identifier a call names, or "" for a call that does
// not reach Ginkgo: a bare It in a file dot-importing Ginkgo, or ginkgo.It
// qualified by a name the file imports Ginkgo under. A method call such
// as r.Context() is not a container however it is named, and nor
// is a builder-named function declared outside Ginkgo. The one call misread
// is through a local declaration shadowing an imported name.
func (g ginkgoImports) callName(expr ast.Expr) string {
	switch fun := expr.(type) {
	case *ast.Ident:
		if g.dot {
			return fun.Name
		}
	case *ast.SelectorExpr:
		if pkg, ok := fun.X.(*ast.Ident); ok && slices.Contains(g.names, pkg.Name) {
			return fun.Sel.Name
		}
	}
	return ""
}
