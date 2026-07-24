package testlayout

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// specBuilders are the Ginkgo container and leaf builders. A call to one marks
// the file as spec-bearing and the package as using Ginkgo, which is what puts
// it under the adapter rules.
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

// errUnparsed marks a test file the pass does not hold that will not parse.
// What such a file declares is unknown, and the safe reading of an unknown file
// is that it might hold the adapter.
var errUnparsed = errors.New("test file does not parse")

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
	var found info
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			if node.Recv == nil && isTestFuncName(node.Name.Name) {
				found.specs = true
			}
		case *ast.CallExpr:
			switch name := callName(node.Fun); {
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

// inspectPath reads a test file the pass does not hold. Reading goes through
// the filesystem rather than [analysis.Pass.ReadFile], which serves only
// the files of its own pass. A file that will not parse yields [errUnparsed]:
// the driver reports the syntax error against the package holding the file,
// and that package is skipped, so this pass is the one left judging a file
// it cannot read.
//
// [analysis.Pass.ReadFile]: https://pkg.go.dev/golang.org/x/tools/go/analysis#Pass.ReadFile
func inspectPath(path string) (info, error) {
	src, err := os.ReadFile(path) //nolint:gosec // the path is a test file in a directory the pass is already analyzing
	if err != nil {
		return info{}, fmt.Errorf("reading %s: %w", path, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		return info{}, fmt.Errorf("%s: %w", path, errUnparsed)
	}
	return inspect(file), nil
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

// callName is the identifier a call names, whether the package holding it was
// dot-imported (It) or not (ginkgo.It).
func callName(expr ast.Expr) string {
	switch fun := expr.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}
