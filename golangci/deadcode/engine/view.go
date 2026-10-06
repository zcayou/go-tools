package engine

import (
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// A view is the evidence policy one evaluation runs under. The full view admits
// every fact the loaded program holds. The masked view removes test-origin
// evidence — facts positioned in _test.go files, the packages that exist only
// because of tests, and the packages declared test-facing — so its verdicts
// answer whether the program needs a declaration with no test vouching for it.
// The program itself is shared: masking holds the loaded set constant, so
// the two evaluations differ in exactly one variable.
type view struct {
	masked bool
	// testFacing is the declared test-facing set, packages whose facts
	// are test-origin by declaration. Only the masked view consults it.
	testFacing map[string]bool
	// testVariants holds the type-checked packages that exist only because
	// of tests, which is how a synthesized function with no position of its own
	// is attributed. Only the masked view consults it.
	testVariants map[*types.Package]bool
	// consumers is what lets tests stand in for the declared API's consumers,
	// nil unless [Config.APITestConsumers] is set. Only the masked view consults
	// it: test-origin evidence aimed at the API's exported surface is admitted
	// there as production evidence would be.
	consumers *testConsumers
}

// testConsumers is what the masked view needs to judge test evidence
// as a consumer's.
type testConsumers struct {
	// api holds the declared API packages.
	api map[string]bool
	// analyzed holds the packages this run reports on. A type declared anywhere
	// else is a dependency's, which a consumer can name as freely as a test can.
	analyzed map[string]bool
	// instantiations says whether a test's instantiation of an API generic
	// stands in for a consumer's. [GenericRootingSkip] declines that reading.
	instantiations bool
}

// maskedView is the view that removes test-origin evidence from the loaded
// program, with testFacing declared test origin as well. consumers, when not
// nil, is the API whose test-origin evidence it admits nonetheless.
func maskedView(pkgs []*packages.Package, testFacing map[string]bool, consumers *testConsumers) view {
	variants := map[*types.Package]bool{}
	for _, pkg := range pkgs {
		if pkg.Types != nil && testVariant(pkg) {
			variants[pkg.Types] = true
		}
	}
	return view{masked: true, testFacing: testFacing, testVariants: variants, consumers: consumers}
}

// admitsReference reports whether a reference to obj, written in the named file
// of the package at path, counts as evidence.
func (v view) admitsReference(path, file string, obj types.Object) bool {
	return v.admitsPackage(path) && v.admitsFile(file) || v.consumerObject(obj)
}

// admitsSelection reports whether a method selection, written in the named file
// of the package at path, counts as evidence. A consumer selects through
// the type it holds, so the selection's receiver decides rather than the type
// declaring the method: a method promoted onto an API type from an unexported
// one is that API type's method.
func (v view) admitsSelection(path, file string, selection *types.Selection) bool {
	if v.admitsPackage(path) && v.admitsFile(file) {
		return true
	}
	return selection.Obj().Exported() && v.consumerType(selection.Recv())
}

// admitsConversion reports whether a conversion of a value of operand type
// to iface, performed by instr in fn, counts as evidence: to an interface
// from a concrete type or from another interface alike. A test's conversion
// stands in for a consumer's when a consumer could write it — both ends
// are types it can name — and it is aimed at the API, an exported API type
// at one end or the other. A consumer puts an API type behind any interface,
// its own included, and puts its own types behind an API interface, but never
// holds a production type the API does not export, so a test converting one
// is test evidence still.
func (v view) admitsConversion(fset *token.FileSet, fn *ssa.Function, instr ssa.Instruction, operand, iface types.Type) bool {
	if v.admitsInstruction(fset, fn, instr) {
		return true
	}
	return (v.consumerType(operand) || v.consumerType(iface)) &&
		v.consumerWritable(fset, operand) && v.consumerWritable(fset, iface)
}

// admitsInstances reports whether any instantiation site the view does not
// admit can stand in for a consumer's.
func (v view) admitsInstances() bool {
	return v.consumers != nil && v.consumers.instantiations
}

// admitsInstance reports whether a test's instantiation site, written
// in a file or package the view does not admit, stands in for a consumer's.
// The instantiated generic has to be one a consumer reaches — an exported
// generic of an API package, or an exported generic method selected through
// an exported API type, promoted ones included, as [view.admitsSelection]
// judges one — and every type argument one a consumer could write. A test
// instantiating an API generic with a production type the API does not export
// supplies an argument no consumer can.
func (v view) admitsInstance(fset *token.FileSet, used types.Object, selection *types.Selection, args []types.Type) bool {
	if !v.admitsInstances() {
		return false
	}
	for _, arg := range args {
		if !v.consumerWritable(fset, arg) {
			return false
		}
	}
	if fn, ok := used.(*types.Func); ok && fn.Signature().Recv() != nil {
		return selection != nil && fn.Exported() && v.consumerType(selection.Recv())
	}
	return v.consumerObject(used)
}

// consumerObject reports whether obj is an exported package-level declaration
// of an API package whose tests stand in for its consumers.
func (v view) consumerObject(obj types.Object) bool {
	return v.consumers != nil && obj.Exported() && obj.Pkg() != nil && v.consumers.api[obj.Pkg().Path()]
}

// consumerType reports whether t, through a pointer, is an exported named type
// of an API package whose tests stand in for its consumers.
func (v view) consumerType(t types.Type) bool {
	if v.consumers == nil {
		return false
	}
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, ok := t.(*types.Named)
	return ok && v.consumerObject(named.Origin().Obj())
}

// consumerInterface reports whether the named interface is declared by test
// code standing in for a consumer: in a _test.go file or a declared test-facing
// package, while the view admits API test consumers at all. A consumer's own
// interface over a public type has its call sites in the consumer, so
// it confers the way a dependency's does.
func (v view) consumerInterface(fset *token.FileSet, named *types.Named) bool {
	return v.consumers != nil && v.testDeclared(fset, named.Obj())
}

// testDeclared reports whether obj is declared by test code: in a _test.go file
// or a declared test-facing package.
func (v view) testDeclared(fset *token.FileSet, obj types.Object) bool {
	if obj.Pkg() != nil && v.testFacing[obj.Pkg().Path()] {
		return true
	}
	return testFile(position(fset, obj.Pos()).Filename)
}

// consumerWritable reports whether a consumer could write t: every named type
// it is built from is an exported type of an API package, a type test code
// declares, or one declared outside the packages this run reports on, and none
// is a type parameter, which only a generic body sees.
func (v view) consumerWritable(fset *token.FileSet, t types.Type) bool {
	if v.consumers == nil {
		return false
	}
	t = types.Unalias(t)
	switch t := t.(type) {
	case *types.Basic:
		return true
	case *types.TypeParam:
		return false
	case *types.Named:
		obj := t.Origin().Obj()
		pkg := obj.Pkg()
		writable := pkg == nil || !v.consumers.analyzed[pkg.Path()] || v.consumerObject(obj) ||
			v.testDeclared(fset, obj)
		return writable && v.allWritable(fset, slices.Collect(t.TypeArgs().Types()))
	}
	parts, ok := typeParts(t)
	return ok && v.allWritable(fset, parts)
}

func (v view) allWritable(fset *token.FileSet, parts []types.Type) bool {
	for _, t := range parts {
		if !v.consumerWritable(fset, t) {
			return false
		}
	}
	return true
}

// typeParts returns the types an unnamed composite type is written from,
// and false for a type that is not one.
func typeParts(t types.Type) ([]types.Type, bool) {
	switch t := t.(type) {
	case *types.Pointer:
		return []types.Type{t.Elem()}, true
	case *types.Slice:
		return []types.Type{t.Elem()}, true
	case *types.Array:
		return []types.Type{t.Elem()}, true
	case *types.Chan:
		return []types.Type{t.Elem()}, true
	case *types.Map:
		return []types.Type{t.Key(), t.Elem()}, true
	case *types.Signature:
		return slices.Concat(tupleTypes(t.Params()), tupleTypes(t.Results())), true
	case *types.Struct:
		parts := make([]types.Type, 0, t.NumFields())
		for field := range t.Fields() {
			parts = append(parts, field.Type())
		}
		return parts, true
	case *types.Interface:
		var parts []types.Type
		for method := range t.ExplicitMethods() {
			parts = append(parts, method.Type())
		}
		return append(parts, slices.Collect(t.EmbeddedTypes())...), true
	}
	return nil, false
}

func tupleTypes(tuple *types.Tuple) []types.Type {
	parts := make([]types.Type, 0, tuple.Len())
	for v := range tuple.Variables() {
		parts = append(parts, v.Type())
	}
	return parts
}

// admitsFile reports whether facts read from the named file count as evidence.
func (v view) admitsFile(name string) bool {
	return !v.masked || !testFile(name)
}

// admitsPackage reports whether the package's facts count as evidence.
// The synthesized test main needs the suffix check — its generated file is not
// named like a test file, while every other test-variant file is —
// and a declared test-facing package is test origin by its path alone.
func (v view) admitsPackage(path string) bool {
	return !v.masked || (!strings.HasSuffix(path, ".test") && !v.testFacing[path])
}

// admitsRoots reports whether the package may contribute call-graph roots.
// Under the masked view neither a test variant nor a declared test-facing
// package roots anything: what only they reach is exactly what the mask exists
// to expose.
func (v view) admitsRoots(pkg *packages.Package) bool {
	return !v.masked || (!testVariant(pkg) && !v.testFacing[pkg.PkgPath])
}

// admitsFunction reports whether fn's own declaration counts under the view:
// written in a file it admits, in a package it admits. An instantiation
// belongs to no package, so the generic it was instantiated from answers
// for it, whose position the instantiation already carries.
func (v view) admitsFunction(fset *token.FileSet, fn *ssa.Function) bool {
	if !v.masked {
		return true
	}
	declared := fn
	if origin := fn.Origin(); origin != nil {
		declared = origin
	}
	if declared.Pkg != nil && !v.admitsPackage(declared.Pkg.Pkg.Path()) {
		return false
	}
	return v.admitsFile(position(fset, fn.Pos()).Filename)
}

// admitsInstruction reports whether an instruction of fn counts as evidence.
// The instruction's own position decides where one exists — code a test file
// contributes to a shared package initializer carries its file's name there —
// and the enclosing function stands in where it does not: a synthetic wrapper
// without a position of its own is attributed to the function it wraps.
//
// Where neither places it, the package does. SSA positions an implicit
// conversion nowhere, and a package's synthesized initializer has no position
// either, so a conversion a _test.go file's package-level initializer performs
// — a table of entries boxed into ...any — would otherwise count as production
// evidence. A test variant's initializer is test code for this purpose:
// the in-package variant's runs the production files' initializers as well, but
// the plain package's own initializer runs those too, and it is admitted.
func (v view) admitsInstruction(fset *token.FileSet, fn *ssa.Function, instr ssa.Instruction) bool {
	if !v.masked {
		return true
	}
	if fn.Pkg != nil && !v.admitsPackage(fn.Pkg.Pkg.Path()) {
		return false
	}
	if pos := instr.Pos(); pos.IsValid() {
		return v.admitsFile(position(fset, pos).Filename)
	}
	if pos := fn.Pos(); pos.IsValid() {
		return v.admitsFile(position(fset, pos).Filename)
	}
	if obj := fn.Object(); obj != nil && obj.Pos().IsValid() {
		return v.admitsFile(position(fset, obj.Pos()).Filename)
	}
	return fn.Pkg == nil || !v.testVariants[fn.Pkg.Pkg]
}

// testVariant reports whether the loaded package exists only because of tests:
// the synthesized test main, an external test package, or the in-package
// variant recompiled with its _test.go files. The in-package variant shares its
// import path with the plain package while its initializer runs the test files'
// inits, so identity is decided by the files a package holds rather than by its
// path.
func testVariant(pkg *packages.Package) bool {
	if strings.HasSuffix(pkg.PkgPath, ".test") || strings.HasSuffix(pkg.PkgPath, "_test") {
		return true
	}
	return slices.ContainsFunc(pkg.CompiledGoFiles, testFile)
}

func testFile(name string) bool {
	return strings.HasSuffix(name, "_test.go")
}
