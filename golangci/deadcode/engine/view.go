package engine

import (
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// A view is the evidence policy one evaluation runs under. The full view admits
// every fact the loaded program holds. The masked view removes test-origin
// evidence — facts positioned in _test.go files, and the packages that exist
// only because of tests — so its verdicts answer whether the program needs
// a declaration with no test vouching for it. The program itself is shared:
// masking holds the loaded set constant, so the two evaluations differ
// in exactly one variable.
type view struct {
	masked bool
}

// admitsFile reports whether facts read from the named file count as evidence.
func (v view) admitsFile(name string) bool {
	return !v.masked || !testFile(name)
}

// admitsPackage reports whether the package's facts count as evidence. Only
// the synthesized test main needs a path check: its generated file is not named
// like a test file, while every other test-variant file is.
func (v view) admitsPackage(path string) bool {
	return !v.masked || !strings.HasSuffix(path, ".test")
}

// admitsInstruction reports whether an instruction of fn counts as evidence.
// The instruction's own position decides where one exists — code a test file
// contributes to a shared package initializer carries its file's name there —
// and the enclosing function stands in where it does not: a synthetic wrapper
// without a position of its own is attributed to the function it wraps.
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
	return true
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
