package zassert

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/zcayou/go-tools/golangci/internal/enum"
)

// errorType is the error interface, which a value's type implements to make
// the value an error.
var errorType, _ = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

// isError reports whether expr's type implements error.
func (p *passChecker) isError(expr ast.Expr) bool {
	return implements(p.pass.TypesInfo.TypeOf(expr), errorType)
}

// isContext reports whether expr's type implements context.Context.
func (p *passChecker) isContext(expr ast.Expr) bool {
	return p.contextType() != nil && implements(p.pass.TypesInfo.TypeOf(expr), p.contextType())
}

func implements(t types.Type, iface *types.Interface) bool {
	if t == nil {
		return false
	}
	if basic, ok := t.(*types.Basic); ok && basic.Info()&types.IsUntyped != 0 {
		return false
	}
	return types.Implements(t, iface)
}

// contextType returns context.Context when the pass's package imports
// the context package, directly or transitively, or nil when it doesn't
// and no value can be one.
func (p *passChecker) contextType() *types.Interface {
	if p.contextRead {
		return p.context
	}
	p.contextRead = true
	seen := map[*types.Package]bool{}
	var find func(pkg *types.Package) *types.Interface
	find = func(pkg *types.Package) *types.Interface {
		if seen[pkg] {
			return nil
		}
		seen[pkg] = true
		if pkg.Path() == "context" {
			if obj, ok := pkg.Scope().Lookup("Context").(*types.TypeName); ok {
				iface, _ := obj.Type().Underlying().(*types.Interface)
				return iface
			}
			return nil
		}
		for _, imported := range pkg.Imports() {
			if iface := find(imported); iface != nil {
				return iface
			}
		}
		return nil
	}
	p.context = find(p.pass.Pkg)
	return p.context
}

// variableOf returns the variable expr names, or nil.
func (p *passChecker) variableOf(expr ast.Expr) (*types.Var, *variable) {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return nil, nil
	}
	v, ok := p.pass.TypesInfo.Uses[id].(*types.Var)
	if !ok || p.variables[v] == nil {
		return nil, nil
	}
	return v, p.variables[v]
}

// isCause reports whether expr is a cause: errors.Unwrap of an error,
// or a variable every write to which is one, each made in the function
// declaring the variable. A write from a nested function literal may run
// at any time, so it is a write that cannot be ordered.
func (p *passChecker) isCause(expr ast.Expr) bool {
	if call, ok := ast.Unparen(expr).(*ast.CallExpr); ok {
		name, ok := p.refers(call.Fun, errorsPath)
		return ok && name == "Unwrap"
	}
	key, v := p.variableOf(expr)
	if v == nil {
		return false
	}
	if cause, ok := p.causes[key]; ok {
		return cause
	}
	// A variable written from itself reads as no cause while it is being read.
	p.causes[key] = false
	cause := len(v.writes) > 0
	for _, w := range v.writes {
		cause = cause && w.value != nil && w.index < 0 && w.fn == v.fn && p.isCause(w.value)
	}
	p.causes[key] = cause
	return cause
}

// stringTransformations are the strings functions returning a string derived
// from their first argument, so that text passed through one is still text.
var stringTransformations = []string{
	"Clone", "Replace", "ReplaceAll", "Title", "ToLower", "ToTitle", "ToUpper", "ToValidUTF8",
	"Trim", "TrimFunc", "TrimLeft", "TrimLeftFunc", "TrimPrefix", "TrimRight", "TrimRightFunc", "TrimSpace",
	"TrimSuffix",
}

// textOf returns the errors whose text expr is:
//   - Error() called on an error, of that error;
//   - an fmt.Sprint, Sprintf or Sprintln, of its error operands;
//   - a strings transformation, a slice, or a concatenation of such text;
//   - a variable every write to which is such text.
func (p *passChecker) textOf(expr ast.Expr) ([]ast.Expr, bool) {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return p.variableText(e)
	case *ast.SliceExpr:
		return p.textOf(e.X)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return nil, false
		}
		x, xok := p.textOf(e.X)
		y, yok := p.textOf(e.Y)
		return append(x, y...), xok || yok
	case *ast.CallExpr:
		if receiver, ok := p.errorText(e); ok {
			return []ast.Expr{receiver}, true
		}
		if name, ok := p.refers(e.Fun, fmtPath); ok && slices.Contains(sprints, name) {
			operands := e.Args
			if name == "Sprintf" && len(operands) > 0 {
				operands = operands[1:]
			}
			errs := slices.DeleteFunc(slices.Clone(operands), func(operand ast.Expr) bool { return !p.isError(operand) })
			return errs, len(errs) > 0
		}
		if p.stringTransformation(e.Fun) && len(e.Args) > 0 {
			return p.textOf(e.Args[0])
		}
	}
	return nil, false
}

// errorText returns the error a call reads the text of: Error() called
// on an error.
func (p *passChecker) errorText(call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || len(call.Args) != 0 || sel.Sel.Name != "Error" || !p.isError(sel.X) {
		return nil, false
	}
	return sel.X, true
}

// textResult is what [passChecker.variableText] found of one variable.
type textResult struct {
	subjects []ast.Expr
	text     bool
}

// variableText reports whether every write to a variable, and there is one,
// is an error's text, and of which errors.
func (p *passChecker) variableText(id *ast.Ident) ([]ast.Expr, bool) {
	key, v := p.variableOf(id)
	if v == nil {
		return nil, false
	}
	if found, ok := p.texts[key]; ok {
		return found.subjects, found.text
	}
	p.texts[key] = textResult{}

	found := textResult{text: len(v.writes) > 0}
	for _, w := range v.writes {
		if w.value == nil || w.index >= 0 {
			found = textResult{}
			break
		}
		subjects, ok := p.textOf(w.value)
		if !ok {
			found = textResult{}
			break
		}
		found.subjects = append(found.subjects, subjects...)
	}
	p.texts[key] = found
	return found.subjects, found.text
}

// stringTransformation reports whether expr is one of the strings functions
// that derive a string from the string they are given.
func (p *passChecker) stringTransformation(expr ast.Expr) bool {
	name, ok := p.refers(expr, stringsPath)
	return ok && slices.Contains(stringTransformations, name)
}

// enumRef is an enum a spec meets, as its fact describes it.
type enumRef struct {
	named *types.Named
	fact  *enumFact
}

// display is the enum as a message names it: qualified by its package's
// name, as source code spells it.
func (e enumRef) display() string {
	obj := e.named.Obj()
	return obj.Pkg().Name() + "." + obj.Name()
}

// enumOf returns the enum t is, declared in the spec's own module, or false.
func (p *passChecker) enumOf(t types.Type) (enumRef, bool) {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return enumRef{}, false
	}
	fact := new(enumFact)
	if !p.pass.ImportObjectFact(named.Obj(), fact) || fact.Root == "" || fact.Root != p.root {
		return enumRef{}, false
	}
	return enumRef{named: named, fact: fact}, true
}

// renderingOf returns the enums whose rendering expr is:
//   - v.String(), with an enum's own String method;
//   - string(v) of a string enum;
//   - fmt.Sprint of an enum value;
//   - a strings transformation or a slice of a rendering;
//   - a variable every write to which is a rendering.
func (p *passChecker) renderingOf(expr ast.Expr) []enumRef {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return p.variableRendering(e)
	case *ast.SliceExpr:
		return p.renderingOf(e.X)
	case *ast.CallExpr:
		if tv, ok := p.pass.TypesInfo.Types[e.Fun]; ok && tv.IsType() {
			return p.conversionRendering(e, tv.Type)
		}
		if ref, ok := p.stringMethod(p.funcOf(e.Fun)); ok {
			return []enumRef{ref}
		}
		if name, ok := p.refers(e.Fun, fmtPath); ok && name == "Sprint" {
			var refs []enumRef
			for _, arg := range e.Args {
				if ref, ok := p.enumOf(p.pass.TypesInfo.TypeOf(arg)); ok {
					refs = append(refs, ref)
				}
			}
			return refs
		}
		if p.stringTransformation(e.Fun) && len(e.Args) > 0 {
			return p.renderingOf(e.Args[0])
		}
	}
	return nil
}

// conversionRendering returns the string enum a conversion to a string type
// renders, if it converts one.
func (p *passChecker) conversionRendering(call *ast.CallExpr, to types.Type) []enumRef {
	if len(call.Args) != 1 {
		return nil
	}
	from := p.pass.TypesInfo.TypeOf(call.Args[0])
	ref, ok := p.enumOf(from)
	if !ok || types.Identical(from, to) || !stringType(ref.named) || !stringType(to) {
		return nil
	}
	return []enumRef{ref}
}

func stringType(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

// stringMethod returns the enum fn is the String method of: the one on a value
// receiver that publishes each member's name.
func (p *passChecker) stringMethod(fn *types.Func) (enumRef, bool) {
	if fn == nil || fn.Name() != "String" {
		return enumRef{}, false
	}
	recv := fn.Signature().Recv()
	if recv == nil {
		return enumRef{}, false
	}
	ref, ok := p.enumOf(recv.Type())
	if !ok || enum.Name(ref.named) != fn.Origin() {
		return enumRef{}, false
	}
	return ref, true
}

// variableRendering returns the enums a variable renders when every write
// to it, and there is one, is a rendering.
func (p *passChecker) variableRendering(id *ast.Ident) []enumRef {
	key, v := p.variableOf(id)
	if v == nil {
		return nil
	}
	if found, ok := p.renderings[key]; ok {
		return found
	}
	p.renderings[key] = nil

	var found []enumRef
	for _, w := range v.writes {
		if w.value == nil || w.index >= 0 {
			found = nil
			break
		}
		refs := p.renderingOf(w.value)
		if len(refs) == 0 {
			found = nil
			break
		}
		found = append(found, refs...)
	}
	p.renderings[key] = found
	return found
}

// fieldRendering returns the enum HaveField's path renders on a value of type
// t, when the path ends in String() on an enum.
func (p *passChecker) fieldRendering(t types.Type, path string) (enumRef, bool) {
	segments := strings.Split(path, ".")
	last := len(segments) - 1
	if segments[last] != "String()" {
		return enumRef{}, false
	}
	for _, segment := range segments[:last] {
		if t = p.member(t, segment); t == nil {
			return enumRef{}, false
		}
	}
	obj, _, _ := types.LookupFieldOrMethod(t, true, p.pass.Pkg, "String")
	method, ok := obj.(*types.Func)
	if !ok {
		return enumRef{}, false
	}
	return p.stringMethod(method)
}

// member returns the type of the field, or the result of the method, that one
// segment of a HaveField path names on a value of type t, or nil.
func (p *passChecker) member(t types.Type, segment string) types.Type {
	name, method := strings.CutSuffix(segment, "()")
	obj, _, _ := types.LookupFieldOrMethod(t, true, p.pass.Pkg, name)
	switch obj := obj.(type) {
	case *types.Var:
		if !method {
			return obj.Type()
		}
	case *types.Func:
		if results := obj.Signature().Results(); method && results.Len() == 1 {
			return results.At(0).Type()
		}
	}
	return nil
}
