package wholemodule

import (
	"bytes"
	"go/token"
	"go/types"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"

	"golang.org/x/tools/go/gcexportdata"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// loadMode reads the module's own packages from source with full type
// information and every dependency from export data. Without NeedDeps
// the loader type checks nothing but the module, which is what makes this load
// cheaper than deadcode's: no rule that shares it needs a dependency's bodies.
const loadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo |
	packages.NeedTypesSizes

// graphMode lists every package the module imports, directly or transitively,
// with its export data, which is what the typed load cannot say: export data
// names only the packages its exported declarations refer to, and holds those
// only as far as the declarations refer to them.
const graphMode = packages.NeedName | packages.NeedImports | packages.NeedDeps | packages.NeedExportFile

// Module is one typed load of a module's own packages, every package ./...
// matches from the module root with test files excluded, and the complete
// type information of every package they import, directly or transitively.
// It does not change once [Load] returns it, so analyses may read it at once.
type Module struct {
	// Root is the module root the load ran in.
	Root string

	// Fset holds every position the load produced.
	Fset *token.FileSet

	// Packages are the module packages, each with syntax and type information,
	// in the order the loader returned them.
	Packages []*packages.Package

	byTypes map[*types.Package]*packages.Package

	// imported holds the import path of every foreign package the module
	// imports, directly or transitively.
	imported map[string]bool

	// known is the type information of every package the module declares
	// or imports, by import path, each one complete.
	known map[string]*types.Package

	ssaOnce sync.Once
	ssa     *Program
}

// Program is the SSA form of a [Module], built over its packages with every
// generic instantiated, so a generic function is read once per instantiation
// the module builds.
type Program struct {
	*ssa.Program

	// Packages maps each module package to its SSA package.
	Packages map[*types.Package]*ssa.Package
}

// loads memoizes [Load] for the process, keyed by module root and build tags,
// so linters sharing a configuration share one load.
var loads = struct {
	sync.Mutex
	entries map[string]*loadEntry
}{entries: map[string]*loadEntry{}}

type loadEntry struct {
	once   sync.Once
	module *Module
	err    error
}

// Load returns the typed load of the module rooted at root under buildTags,
// loading it on the first request for that root and tag set and returning
// the same result to every later one. A package that fails to load or type
// check fails the load, as does an imported package whose export data cannot
// be read.
func Load(root string, buildTags []string) (*Module, error) {
	tags := slices.Clone(buildTags)
	slices.Sort(tags)
	key := root + "\x00" + strings.Join(tags, ",")

	loads.Lock()
	entry := loads.entries[key]
	if entry == nil {
		entry = new(loadEntry)
		loads.entries[key] = entry
	}
	loads.Unlock()
	entry.once.Do(func() { entry.module, entry.err = load(root, tags) })
	return entry.module, entry.err
}

func load(root string, buildTags []string) (*Module, error) {
	var buildFlags []string
	if len(buildTags) > 0 {
		buildFlags = []string{"-tags=" + strings.Join(buildTags, ",")}
	}
	pkgs, err := packages.Load(&packages.Config{Mode: loadMode, Dir: root, BuildFlags: buildFlags}, "./...")
	if err != nil {
		return nil, &LoadError{cause: err}
	}

	if failed := packageErrors(pkgs); len(failed) > 0 {
		return nil, &LoadError{Errors: failed}
	}

	module := &Module{
		Root:     root,
		Packages: pkgs,
		byTypes:  make(map[*types.Package]*packages.Package, len(pkgs)),
		known:    map[string]*types.Package{},
	}
	if len(pkgs) > 0 {
		module.Fset = pkgs[0].Fset
	} else {
		module.Fset = token.NewFileSet()
	}
	for _, pkg := range pkgs {
		module.byTypes[pkg.Types] = pkg
		module.know(pkg.Types)
	}
	if err := module.complete(root, buildFlags); err != nil {
		return nil, err
	}
	return module, nil
}

// know records pkg and every package its type information refers to.
func (m *Module) know(pkg *types.Package) {
	if m.known[pkg.Path()] != nil {
		return
	}
	m.known[pkg.Path()] = pkg
	for _, imported := range pkg.Imports() {
		m.know(imported)
	}
}

// complete lists every foreign package the module's packages import, directly
// or transitively, and reads the export data of each one the typed load does
// not hold complete, dependencies first, into the load's own type universe.
// A package the load holds only as far as other export data refers
// to it is completed in place, so its types stay identical to the load's own
// wherever the two meet. It fails with a [*LoadError].
func (m *Module) complete(root string, buildFlags []string) error {
	pkgs, err := packages.Load(&packages.Config{Mode: graphMode, Dir: root, BuildFlags: buildFlags}, "./...")
	if err != nil {
		return &LoadError{cause: err}
	}
	m.imported = map[string]bool{}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		if err != nil || m.declares(pkg.PkgPath) {
			return
		}
		m.imported[pkg.PkgPath] = true
		switch known := m.known[pkg.PkgPath]; {
		case known != nil && known.Complete():
		case pkg.PkgPath == "unsafe":
			// unsafe has no export data: the type checker supplies it.
			m.known[pkg.PkgPath] = types.Unsafe
		default:
			err = m.readExport(pkg)
		}
	})
	return err
}

// declares reports whether path is a module package's.
func (m *Module) declares(path string) bool {
	known := m.known[path]
	return known != nil && m.byTypes[known] != nil
}

// readExport reads a package from its export data into the known packages,
// resolving each package it refers to against them. It fails
// with a [*LoadError].
func (m *Module) readExport(pkg *packages.Package) error {
	if pkg.ExportFile == "" {
		return &LoadError{Package: pkg.PkgPath, Errors: packageErrors([]*packages.Package{pkg})}
	}
	data, err := os.ReadFile(pkg.ExportFile)
	if err != nil {
		return &LoadError{Package: pkg.PkgPath, cause: err}
	}
	//nolint:staticcheck // go/packages reads export files the same way; x/tools offers no other reader
	reader, err := gcexportdata.NewReader(bytes.NewReader(data))
	if err != nil {
		return &LoadError{Package: pkg.PkgPath, cause: err}
	}
	if _, err := gcexportdata.Read(reader, m.Fset, m.known, pkg.PkgPath); err != nil {
		return &LoadError{Package: pkg.PkgPath, cause: err}
	}
	return nil
}

// LoadError is a module, or one package it imports, that could not be loaded.
type LoadError struct {
	// Package is the imported package whose export data could not be read,
	// or empty when the module's own packages failed to load.
	Package string

	// Errors holds what the loader said of each package that failed to load
	// or type check, by import path. It is empty when the loader itself failed.
	Errors map[string][]packages.Error

	// cause is the loader's own failure.
	cause error
}

func (e *LoadError) Error() string {
	subject := "packages"
	if e.Package != "" {
		subject = e.Package
	}
	if e.cause != nil {
		return "loading " + subject + ": " + e.cause.Error()
	}
	var lines []string
	for _, path := range slices.Sorted(maps.Keys(e.Errors)) {
		for _, err := range e.Errors[path] {
			lines = append(lines, path+": "+err.Error())
		}
	}
	if len(lines) == 0 {
		return "loading " + subject + " failed, and the loader said nothing of why"
	}
	return "packages contain errors: " + strings.Join(lines, "; ")
}

// Unwrap returns the loader's own failure, when it failed.
func (e *LoadError) Unwrap() error {
	return e.cause
}

// Package returns the module package declaring pkg, or nil for a foreign one.
func (m *Module) Package(pkg *types.Package) *packages.Package {
	return m.byTypes[pkg]
}

// ImportedPaths returns the import path of every foreign package the module's
// packages import, directly or transitively, sorted.
func (m *Module) ImportedPaths() []string {
	return slices.Sorted(maps.Keys(m.imported))
}

// ImportedPackage returns the complete type information of a package
// the module's packages declare or import, directly or transitively, or nil
// for any other path.
func (m *Module) ImportedPackage(path string) *types.Package {
	if !m.imported[path] && !m.declares(path) {
		return nil
	}
	return m.known[path]
}

// packageErrors returns what the loader said of each package, by import path.
func packageErrors(pkgs []*packages.Package) map[string][]packages.Error {
	failed := map[string][]packages.Error{}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			failed[pkg.PkgPath] = pkg.Errors
		}
	}
	return failed
}

// Declares reports whether obj is declared in a module package.
func (m *Module) Declares(obj types.Object) bool {
	return obj != nil && obj.Pkg() != nil && m.byTypes[obj.Pkg()] != nil
}

// Program returns the module's SSA program, building it on the first request.
// Each module package is built from its syntax; each package one of them
// imports directly is created from its type information, and the builder
// creates what the bodies need of the rest on demand.
func (m *Module) Program() *Program {
	m.ssaOnce.Do(func() {
		prog := ssa.NewProgram(m.Fset, ssa.InstantiateGenerics)
		created := map[*types.Package]bool{}
		for _, pkg := range m.Packages {
			created[pkg.Types] = true
		}
		for _, pkg := range m.Packages {
			for _, imported := range pkg.Types.Imports() {
				if !created[imported] {
					created[imported] = true
					prog.CreatePackage(imported, nil, nil, true)
				}
			}
		}
		ssaPkgs := make(map[*types.Package]*ssa.Package, len(m.Packages))
		for _, pkg := range m.Packages {
			ssaPkgs[pkg.Types] = prog.CreatePackage(pkg.Types, pkg.Syntax, pkg.TypesInfo, true)
		}
		prog.Build()
		m.ssa = &Program{Program: prog, Packages: ssaPkgs}
	})
	return m.ssa
}
