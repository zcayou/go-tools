package spec

import (
	"os"
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/internal/enum"
)

// enumFact marks a type as an enum. The pass over the package declaring
// the type exports it, since only that package's syntax says how its constants
// are declared; the passes whose specs assert the enum import it. Its fields
// are exported because golangci-lint caches facts between runs.
type enumFact struct {
	// Root is the directory of the nearest go.mod above the file declaring
	// the type, which decides whether a spec shares the enum's module.
	Root string

	// TypeFile is the file declaring the type.
	TypeFile string

	// StringFile is the file declaring the type's String method, or empty when
	// it declares none.
	StringFile string
}

func (*enumFact) AFact() {}

func (*enumFact) String() string {
	return "enum"
}

// renderingFact marks an error type whose rendering unpinned-rendering judges.
// The pass over the package declaring the type exports it, since the external
// test package's pass holds none of the package's sources and so meets
// the type only through what it imports.
type renderingFact struct {
	// ErrorFile is the file declaring the type's Error method, which the test
	// files that may pin the rendering are named for.
	ErrorFile string
}

func (*renderingFact) AFact() {}

func (*renderingFact) String() string {
	return "rendering"
}

// pinsFact lists the error types whose rendering a package's own test files
// pin. The pass over a package declaring a [renderingFact] type exports it,
// and the external test package's pass, which imports that package's test
// variant, joins it with the pins its own files make.
type pinsFact struct {
	// Pinned are the names of the types pinned.
	Pinned []string
}

func (*pinsFact) AFact() {}

func (*pinsFact) String() string {
	return "pins"
}

// exportEnums exports a fact for each enum the pass's package declares.
func exportEnums(pass *analysis.Pass) {
	roots := map[string]string{}
	declared := enum.Declared(enum.Package{Fset: pass.Fset, Files: pass.Files, Types: pass.Pkg, Info: pass.TypesInfo})
	for _, e := range declared {
		fact := &enumFact{TypeFile: pass.Fset.PositionFor(e.Type.Obj().Pos(), false).Filename}
		dir := filepath.Dir(fact.TypeFile)
		root, ok := roots[dir]
		if !ok {
			root = moduleRoot(dir)
			roots[dir] = root
		}
		fact.Root = root
		if method := enum.Name(e.Type); method != nil {
			fact.StringFile = pass.Fset.PositionFor(method.Pos(), false).Filename
		}
		pass.ExportObjectFact(e.Type.Obj(), fact)
	}
}

// moduleRoot returns the directory of the nearest go.mod at or above dir,
// or "" when there is none.
func moduleRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
