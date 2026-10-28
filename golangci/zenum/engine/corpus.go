package engine

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/objectpath"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// corpus is the code one reading of the use-site rules judges: the module's
// packages, or the test files of its test packages.
type corpus struct {
	a *analysis

	// judged are the files the syntax rules walk.
	judged []source

	// sources are the files the printers are inferred from: the judged ones,
	// and every plain file a test file can call into.
	sources []source

	// functions are the judged code in SSA form, and inferred the functions
	// the printer inference reads conversions from.
	functions []*ssa.Function
	inferred  []*ssa.Function

	// printers are the module's functions, methods, interface methods, variables
	// and fields that format their operands as fmt does, as the judged code
	// sees them.
	printers map[types.Object]printer
}

// source is files of one package.
type source struct {
	pkg   *packages.Package
	files []*ast.File
}

// plainCorpus is the module's packages, read from [wholemodule.Module.Program].
func (a *analysis) plainCorpus() *corpus {
	sources := make([]source, 0, len(a.pkgs))
	for _, pkg := range a.pkgs {
		sources = append(sources, source{pkg: pkg, files: pkg.Syntax})
	}
	functions := a.functions(a.module.Program(), sources, func(*ssa.Function) bool { return true })
	return &corpus{a: a, judged: sources, sources: sources, functions: functions, inferred: functions}
}

// testCorpus is the test files of the module's test packages, read from
// [wholemodule.Module.TestProgram]. A test variant's other files are its
// package's, judged in plain; its printers are plain's, and those declared
// in test files.
func (a *analysis) testCorpus(plain *corpus) *corpus {
	prog := a.module.TestProgram()
	tests := map[*ssa.Package]bool{}
	judged := make([]source, 0, len(a.module.Tests))
	for _, pkg := range a.module.Tests {
		tests[prog.Packages[pkg.Types]] = true
		judged = append(judged, source{pkg: pkg, files: a.testFiles(pkg)})
	}
	functions := a.functions(prog, judged, func(fn *ssa.Function) bool {
		top := outermost(fn)
		switch {
		case top.Origin() != nil:
			return true
		case top.Synthetic != "":
			return tests[top.Pkg]
		default:
			return a.inTestFile(top.Pos())
		}
	})
	return &corpus{
		a:         a,
		judged:    judged,
		sources:   append(slices.Clone(plain.sources), judged...),
		functions: functions,
		inferred:  slices.Concat(plain.functions, functions),
	}
}

// testFiles returns a test package's test files.
func (a *analysis) testFiles(pkg *packages.Package) []*ast.File {
	var files []*ast.File
	for _, file := range pkg.Syntax {
		if a.inTestFile(file.Pos()) {
			files = append(files, file)
		}
	}
	return files
}

// inTestFile reports whether pos is in a test file.
func (a *analysis) inTestFile(pos token.Pos) bool {
	file := a.module.Fset.File(pos)
	return file != nil && strings.HasSuffix(file.Name(), "_test.go")
}

// functions returns every SSA function of prog holding module code that accept
// admits: each function and method the roots' files declare, their packages'
// initializers, which hold package-level variables' initialization, the methods
// of each module type the program converts to an interface, and what those
// reach that is still module code — closures, and the instantiations of generic
// bodies the module builds.
//
// A method reached only through an interface is called by no instruction,
// so the program's runtime types are what find it, an instantiated method
// of a generic type above all. Building such a method on demand is safe while
// the other linters reading the same load walk the program: [ssa.Program]
// guards it. A method with type parameters of its own has no method value
// to build, and is read only through the instantiations its callers make.
func (a *analysis) functions(
	prog *wholemodule.Program, roots []source, accept func(*ssa.Function) bool,
) []*ssa.Function {
	module := map[*ssa.Package]bool{}
	for _, pkg := range prog.Packages {
		module[pkg] = true
	}

	seen := map[*ssa.Function]bool{}
	var found []*ssa.Function
	var visit func(*ssa.Function)
	visit = func(fn *ssa.Function) {
		if fn == nil || seen[fn] || !moduleCode(fn, module) || !accept(fn) {
			return
		}
		seen[fn] = true
		found = append(found, fn)
		for _, anon := range fn.AnonFuncs {
			visit(anon)
		}
		for _, callee := range referenced(fn) {
			visit(callee)
		}
	}

	for _, root := range roots {
		ssaPkg := prog.Packages[root.pkg.Types]
		visit(ssaPkg.Func("init"))
		for _, file := range root.files {
			for _, decl := range file.Decls {
				if fn, isFunc := decl.(*ast.FuncDecl); isFunc {
					if obj, ok := root.pkg.TypesInfo.Defs[fn.Name].(*types.Func); ok {
						visit(prog.FuncValue(obj))
					}
				}
			}
		}
	}
	for _, t := range prog.RuntimeTypes() {
		if !moduleType(prog, t) {
			continue
		}
		for selection := range types.NewMethodSet(t).Methods() {
			if method, ok := selection.Obj().(*types.Func); ok && method.Signature().TypeParams().Len() > 0 {
				continue
			}
			visit(prog.MethodValue(selection))
		}
	}
	return found
}

// referenced returns the functions fn's instructions name as operands:
// the functions and methods it calls or takes as values.
func referenced(fn *ssa.Function) []*ssa.Function {
	var found []*ssa.Function
	var operands []*ssa.Value
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			operands = instr.Operands(operands[:0])
			for _, operand := range operands {
				if callee, ok := (*operand).(*ssa.Function); ok {
					found = append(found, callee)
				}
			}
		}
	}
	return found
}

// moduleType reports whether t is a defined type a package prog builds
// declares, or a pointer to one, other than an interface.
func moduleType(prog *wholemodule.Program, t types.Type) bool {
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || types.IsInterface(named) {
		return false
	}
	return prog.Packages[named.Obj().Pkg()] != nil
}

// moduleCode reports whether fn's body is module code: whether the function
// it is written in, or the generic it instantiates, belongs to a module
// package. Synthesized wrappers belong to none.
func moduleCode(fn *ssa.Function, module map[*ssa.Package]bool) bool {
	top := outermost(fn)
	if origin := top.Origin(); origin != nil {
		top = origin
	}
	return top.Pkg != nil && module[top.Pkg]
}

// outermost returns the function fn is written in, through its closures.
func outermost(fn *ssa.Function) *ssa.Function {
	for fn.Parent() != nil {
		fn = fn.Parent()
	}
	return fn
}

// canonical returns the module package's own object for obj. A test package
// type checks on its own, so a test variant's copy of a declaration in one
// of its package's files, and the copy export data gives a package compiled
// against a variant, are objects of their own; each maps to the module
// package's, by import path and its path within the package. An object
// no module package declares, as one only a test file declares, maps to itself.
func (a *analysis) canonical(obj types.Object) types.Object {
	if obj == nil || obj.Pkg() == nil || a.module.Package(obj.Pkg()) != nil {
		return obj
	}
	plain := a.plain[obj.Pkg().Path()]
	if plain == nil {
		return obj
	}
	if found, ok := a.canonicals[obj]; ok {
		return found
	}
	found := obj
	if path, err := a.paths.For(obj); err == nil {
		if own, err := objectpath.Object(plain.Types, path); err == nil {
			found = own
		}
	}
	a.canonicals[obj] = found
	return found
}

// home returns the import path of the package pkg's code belongs to: its own,
// or for a test package the path of the package under test.
func home(pkg *packages.Package) string {
	if pkg.ForTest != "" {
		return pkg.ForTest
	}
	return pkg.PkgPath
}
