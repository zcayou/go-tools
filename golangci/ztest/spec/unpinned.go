package spec

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// subject is an error type whose rendering unpinned-rendering judges.
type subject struct {
	obj *types.TypeName

	// errorFile is the file declaring the type's Error method.
	errorFile string

	// method is the position of the Error method's name, or invalid for a type
	// the pass meets only through an import.
	method token.Pos
}

func (s subject) key() typeKey {
	return typeKey{path: s.obj.Pkg().Path(), name: s.obj.Name()}
}

// display is the type as a message names it: qualified by its package's name,
// as source code spells it.
func (s subject) display() string {
	return s.obj.Pkg().Name() + "." + s.obj.Name()
}

// typeKey names a defined type by its package's path and its own name.
type typeKey struct {
	path string
	name string
}

// candidate is a test file a rendering pattern names for a source declaring
// an error type, as read from disk.
type candidate struct {
	name string

	// builds reports whether the file's build constraints hold under the build
	// tags, without which it pins nothing.
	builds bool

	// external reports whether the file declares the external test package.
	external bool
}

// unpinned judges the rendering of every error type the pass declares, or,
// for an external test package, meets in the package it tests. A type needs
// a test file that may pin it, and a pin in one of those files; a pass reports
// what the type lacks only where it holds the file the finding sits in, so each
// finding has one pass to decide it. The pass over a declaring package exports
// the types it judges and the pins its own test files make, for the external
// test package's pass to join with its own.
func (c *checker) unpinned(pass *analysis.Pass, dir *directory, tests map[string]*ast.File,
	pins map[string]map[typeKey]bool,
) error {
	declared := declaredSubjects(pass)
	for _, s := range declared {
		pass.ExportObjectFact(s.obj, &renderingFact{ErrorFile: s.errorFile})
	}
	imported, importedPins := importedSubjects(pass)
	subjects := slices.Concat(declared, imported)
	if len(subjects) == 0 {
		return nil
	}
	if dir == nil {
		dir = &directory{path: filepath.Dir(subjects[0].errorFile)}
	}

	fact := &pinsFact{}
	for _, s := range subjects {
		candidates, err := c.candidates(s.errorFile, dir)
		if errors.Is(err, errUnparsed) {
			continue
		}
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			if s.method.IsValid() {
				pass.Report(diagnostic(s.method, RuleUnpinnedRendering, c.missingMessage(s)))
			}
			continue
		}

		pinned := importedPins[s.obj.Name()] || slices.ContainsFunc(candidates, func(cd candidate) bool {
			return pins[filepath.Join(dir.path, cd.name)][s.key()]
		})
		if pinned && s.method.IsValid() {
			fact.Pinned = append(fact.Pinned, s.obj.Name())
		}
		// The anchor is the first candidate the external test package holds,
		// or the first of all when it holds none.
		anchor := candidates[0]
		if i := slices.IndexFunc(candidates, func(cd candidate) bool { return cd.external }); i >= 0 {
			anchor = candidates[i]
		}
		file := tests[filepath.Join(dir.path, anchor.name)]
		if pinned || file == nil {
			continue
		}
		pass.Report(diagnostic(file.Package, RuleUnpinnedRendering, fmt.Sprintf(
			"%s's rendering is not pinned; assert its exact text on a %s value in this file", s.display(), s.display())))
	}
	if len(declared) > 0 {
		pass.ExportPackageFact(fact)
	}
	return nil
}

// missingMessage says that a type has no test file to pin its rendering in,
// naming the file the first rendering pattern gives the source declaring its
// Error.
func (c *checker) missingMessage(s subject) string {
	file := "test file"
	if len(c.config.Renderings) > 0 {
		file = c.config.Renderings[0].Name(strings.TrimSuffix(filepath.Base(s.errorFile), ".go"))
	}
	return fmt.Sprintf("%s has no %s to pin its rendering in", s.display(), file)
}

// declaredSubjects returns the error types the pass's package declares whose
// rendering is judged: an exported defined type declaring Error() string
// on itself or its pointer, the type and the method each in a file
// that is neither a test nor generated, in a package other than main.
// A generated file renders what its generator decides, and a cgo file
// is generated too.
func declaredSubjects(pass *analysis.Pass) []subject {
	if pass.Pkg.Name() == "main" {
		return nil
	}
	skipped := map[string]bool{}
	for _, file := range pass.Files {
		path := pass.Fset.PositionFor(file.Package, false).Filename
		if strings.HasSuffix(path, testSuffix) || ast.IsGenerated(file) {
			skipped[path] = true
		}
	}

	var subjects []subject
	for _, file := range pass.Files {
		path := pass.Fset.PositionFor(file.Package, false).Filename
		if skipped[path] {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "Error" {
				continue
			}
			method, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func)
			if !ok || !rendersString(method.Signature()) {
				continue
			}
			obj := receiverType(method.Signature())
			if obj == nil || !obj.Exported() || skipped[pass.Fset.PositionFor(obj.Pos(), false).Filename] {
				continue
			}
			subjects = append(subjects, subject{obj: obj, errorFile: path, method: fn.Name.Pos()})
		}
	}
	return subjects
}

// rendersString reports whether a method's signature is Error's: no parameters,
// and a string result.
func rendersString(sig *types.Signature) bool {
	return sig.Params().Len() == 0 && sig.Results().Len() == 1 &&
		types.Identical(sig.Results().At(0).Type(), types.Typ[types.String])
}

// receiverType returns the defined type a method is declared on, through
// a pointer receiver.
func receiverType(sig *types.Signature) *types.TypeName {
	t := sig.Recv().Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return nil
	}
	return named.Origin().Obj()
}

// importedSubjects returns, for an external test package, the error types
// the package it tests declares, known by the fact that package's pass
// exports on each, and the names of those its own test files pin.
func importedSubjects(pass *analysis.Pass) ([]subject, map[string]bool) {
	if !strings.HasSuffix(pass.Pkg.Name(), "_test") {
		return nil, nil
	}
	path, ok := strings.CutSuffix(pass.Pkg.Path(), "_test")
	if !ok {
		return nil, nil
	}
	i := slices.IndexFunc(pass.Pkg.Imports(), func(pkg *types.Package) bool { return pkg.Path() == path })
	if i < 0 {
		return nil, nil
	}
	tested := pass.Pkg.Imports()[i]

	var subjects []subject
	for _, name := range tested.Scope().Names() {
		obj, ok := tested.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		fact := new(renderingFact)
		if pass.ImportObjectFact(obj, fact) {
			subjects = append(subjects, subject{obj: obj, errorFile: fact.ErrorFile})
		}
	}
	pinned := map[string]bool{}
	if fact := new(pinsFact); pass.ImportPackageFact(tested, fact) {
		for _, name := range fact.Pinned {
			pinned[name] = true
		}
	}
	return subjects, pinned
}

// candidates returns the test files of dir that may pin the rendering
// of an error type whose Error errorFile declares: those a rendering pattern
// names for errorFile, under any of its build-constrained variants, whose build
// constraints hold. A candidate that will not parse could be either test
// package, and fails with errUnparsed.
func (c *checker) candidates(errorFile string, dir *directory) ([]candidate, error) {
	if err := dir.load(); err != nil {
		return nil, err
	}
	var found []candidate
	for _, name := range dir.tests {
		sources, err := c.renderedSources(name, dir)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(sources, errorFile) {
			continue
		}
		cd, err := c.candidate(dir, name)
		if err != nil {
			return nil, err
		}
		if cd.builds {
			found = append(found, cd)
		}
	}
	return found, nil
}

// candidate reads one test file of dir: whether its build constraints hold
// under the build tags, and which test package it declares.
func (c *checker) candidate(dir *directory, name string) (candidate, error) {
	if cd, ok := dir.candidates[name]; ok {
		return cd, nil
	}
	ctx := build.Default
	ctx.BuildTags = c.config.BuildTags
	builds, err := ctx.MatchFile(dir.path, name)
	if err != nil {
		return candidate{}, err
	}
	cd := candidate{name: name, builds: builds}
	if builds {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir.path, name), nil, parser.PackageClauseOnly)
		if err != nil {
			return candidate{}, errUnparsed
		}
		cd.external = strings.HasSuffix(file.Name.Name, "_test")
	}
	if dir.candidates == nil {
		dir.candidates = map[string]candidate{}
	}
	dir.candidates[name] = cd
	return cd, nil
}

// gomegaPin records a Gomega assertion that pins an error type's rendering:
// a positive one comparing the text of a value of the type exactly, as Equal
// does given its Error() and MatchError given the value, either matcher
// standing alone or inside And or SatisfyAll and given a string; or one
// that a function calling only the value's Error panics, which an Error
// rendering no text has instead, as Panic or PanicWith standing alone asserts.
func (f *fileChecker) gomegaPin(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return
	}
	if negates, asserts := assertionMethods[sel.Sel.Name]; !asserts || negates {
		return
	}
	actual, ok := f.actualOf(sel.X)
	if !ok {
		return
	}
	if value, ok := f.panicking(actual); ok {
		if matcher, ok := ast.Unparen(call.Args[0]).(*ast.CallExpr); ok {
			if found, ok := f.refers(matcher.Fun, gomegaPath); ok && (found == "Panic" || found == "PanicWith") {
				f.pin(value)
			}
		}
		return
	}
	if text, ok := ast.Unparen(actual).(*ast.CallExpr); ok {
		if value, ok := f.errorText(text); ok {
			if f.exactMatcher(call.Args[0], "Equal") {
				f.pin(value)
			}
			return
		}
	}
	if f.exactMatcher(call.Args[0], "MatchError") {
		f.pin(actual)
	}
}

// exactMatcher reports whether a matcher compares its value with a string
// through the Gomega matcher name, alone or inside And or SatisfyAll, where
// nothing but the matcher decides the assertion.
func (f *fileChecker) exactMatcher(matcher ast.Expr, name string) bool {
	call, ok := ast.Unparen(matcher).(*ast.CallExpr)
	if !ok {
		return false
	}
	found, ok := f.refers(call.Fun, gomegaPath)
	switch {
	case !ok:
		return false
	case found == name:
		return len(call.Args) > 0 && f.isString(call.Args[0])
	case found == "And" || found == "SatisfyAll":
		return slices.ContainsFunc(call.Args, func(arg ast.Expr) bool { return f.exactMatcher(arg, name) })
	}
	return false
}

// testifyPin records a testify assertion that pins an error type's rendering:
// Equal or Exactly comparing a string with the Error() of a value of the type,
// either way round, EqualError of such a value and a string, or Panics
// or PanicsWithValue of a function calling only the value's Error.
func (f *fileChecker) testifyPin(name string, values []ast.Expr) {
	if (name == "Panics" || name == "PanicsWithValue") && len(values) > 0 {
		if value, ok := f.panicking(values[len(values)-1]); ok {
			f.pin(value)
		}
		return
	}
	if len(values) != 2 {
		return
	}
	switch name {
	case "Equal", "Exactly":
		for i, value := range values {
			text, ok := ast.Unparen(value).(*ast.CallExpr)
			if !ok {
				continue
			}
			if x, ok := f.errorText(text); ok && f.isString(values[1-i]) {
				f.pin(x)
			}
		}
	case "EqualError":
		if f.isString(values[1]) {
			f.pin(values[0])
		}
	}
}

// panicking returns the error a function literal calls Error on when that call
// is all its body holds, as a statement or assigned to the blank identifier.
func (f *fileChecker) panicking(fn ast.Expr) (ast.Expr, bool) {
	lit, ok := ast.Unparen(fn).(*ast.FuncLit)
	if !ok || len(lit.Body.List) != 1 {
		return nil, false
	}
	var called ast.Expr
	switch stmt := lit.Body.List[0].(type) {
	case *ast.ExprStmt:
		called = stmt.X
	case *ast.AssignStmt:
		blank, isIdent := stmt.Lhs[0].(*ast.Ident)
		if len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 || !isIdent || blank.Name != "_" {
			return nil, false
		}
		called = stmt.Rhs[0]
	default:
		return nil, false
	}
	call, ok := ast.Unparen(called).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	return f.errorText(call)
}

// pin records that the file pins the rendering of value's type, when
// that is a defined type other than an interface, through any pointer
// and instantiation.
func (f *fileChecker) pin(value ast.Expr) {
	t := f.pass.TypesInfo.TypeOf(value)
	if ptr, ok := types.Unalias(t).(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || types.IsInterface(named) {
		return
	}
	obj := named.Origin().Obj()
	if f.pins[f.path] == nil {
		f.pins[f.path] = map[typeKey]bool{}
	}
	f.pins[f.path][typeKey{path: obj.Pkg().Path(), name: obj.Name()}] = true
}

// isString reports whether expr has string type.
func (p *passChecker) isString(expr ast.Expr) bool {
	t := p.pass.TypesInfo.TypeOf(expr)
	return t != nil && stringType(t)
}
