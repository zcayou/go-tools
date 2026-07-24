package engine_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// typecheckPackages type checks single-file packages in order, later sources
// importing earlier ones by their package clause name, and wraps each the way
// the loader hands packages to the engine — type information, syntax,
// and imports wired — so a component taking loaded packages can be driven
// without a module on disk.
func typecheckPackages(sources ...string) []*packages.Package {
	GinkgoHelper()

	fset := token.NewFileSet()
	byPath := map[string]*packages.Package{}
	pkgs := make([]*packages.Package, 0, len(sources))
	for i, source := range sources {
		file, err := parser.ParseFile(fset, fmt.Sprintf("src%d.go", i), source, parser.SkipObjectResolution)
		Expect(err).NotTo(HaveOccurred())

		// The full set of maps the loader fills, because the SSA builder reads them
		// all.
		info := &types.Info{
			Types:        map[ast.Expr]types.TypeAndValue{},
			Instances:    map[*ast.Ident]types.Instance{},
			Defs:         map[*ast.Ident]types.Object{},
			Uses:         map[*ast.Ident]types.Object{},
			Implicits:    map[ast.Node]types.Object{},
			Selections:   map[*ast.SelectorExpr]*types.Selection{},
			Scopes:       map[ast.Node]*types.Scope{},
			FileVersions: map[*ast.File]string{},
		}
		conf := types.Config{Importer: mapImporter(byPath)}
		typesPkg, err := conf.Check(file.Name.Name, fset, []*ast.File{file}, info)
		Expect(err).NotTo(HaveOccurred())

		imports := map[string]*packages.Package{}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			Expect(err).NotTo(HaveOccurred())
			imports[path] = byPath[path]
		}
		pkg := &packages.Package{
			PkgPath:   file.Name.Name,
			Types:     typesPkg,
			TypesInfo: info,
			Syntax:    []*ast.File{file},
			Fset:      fset,
			Imports:   imports,
		}
		byPath[file.Name.Name] = pkg
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

// mapImporter resolves imports against the packages already checked, which
// is all a self-contained spec source may import.
type mapImporter map[string]*packages.Package

func (m mapImporter) Import(path string) (*types.Package, error) {
	pkg, ok := m[path]
	if !ok {
		return nil, fmt.Errorf("importing %q: no earlier source declares it", path)
	}
	return pkg.Types, nil
}

// lookup finds a package-scope object and asserts it is the sort of object
// the spec needs.
func lookup[T types.Object](pkg *packages.Package, name string) T {
	GinkgoHelper()

	obj, ok := pkg.Types.Scope().Lookup(name).(T)
	Expect(ok).To(BeTrue(), "object %s is missing or not the expected kind", name)
	return obj
}

// namedType finds a package-scope named type.
func namedType(pkg *packages.Package, name string) *types.Named {
	GinkgoHelper()

	named, ok := lookup[*types.TypeName](pkg, name).Type().(*types.Named)
	Expect(ok).To(BeTrue(), "type %s is not a named type", name)
	return named
}

// namedMethod finds a declared method of a package-scope named type.
func namedMethod(pkg *packages.Package, typeName, methodName string) *types.Func {
	GinkgoHelper()

	for method := range namedType(pkg, typeName).Methods() {
		if method.Name() == methodName {
			return method
		}
	}
	Fail("type " + typeName + " declares no method " + methodName)
	return nil
}

// interfaceMethod finds a declared method of a package-scope interface type.
func interfaceMethod(pkg *packages.Package, typeName, methodName string) *types.Func {
	GinkgoHelper()

	iface, ok := namedType(pkg, typeName).Underlying().(*types.Interface)
	Expect(ok).To(BeTrue(), "type %s is not an interface", typeName)
	for method := range iface.ExplicitMethods() {
		if method.Name() == methodName {
			return method
		}
	}
	Fail("interface " + typeName + " declares no method " + methodName)
	return nil
}

// buildSSA builds the typechecked packages into an SSA program the way
// the engine does.
func buildSSA(pkgs []*packages.Package) *ssa.Program {
	GinkgoHelper()

	prog, _ := ssautil.Packages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	return prog
}
