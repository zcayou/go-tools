package testlayout

import (
	"go/ast"
	"go/token"
)

// suiteHooks are the Ginkgo hooks that bracket the whole test binary rather
// than any one spec, which is what makes the suite file their home.
// The Synchronized forms are how a parallel run brackets it, and the Report
// forms bracket it with a report rather than with setup.
var suiteHooks = map[string]bool{
	"BeforeSuite":             true,
	"AfterSuite":              true,
	"SynchronizedBeforeSuite": true,
	"SynchronizedAfterSuite":  true,
	"ReportBeforeSuite":       true,
	"ReportAfterSuite":        true,
}

// suiteDecl classifies one top-level declaration of a suite file: what it is,
// for a diagnostic to name, and whether the file may hold it.
func suiteDecl(decl ast.Decl) (what string, allowed bool) {
	switch decl := decl.(type) {
	case *ast.GenDecl:
		switch decl.Tok {
		case token.IMPORT:
			return "", true
		case token.TYPE:
			return "a type declaration", false
		case token.CONST:
			return "a constant declaration", false
		case token.VAR:
			return suiteVar(decl)
		}
	case *ast.FuncDecl:
		switch {
		case decl.Recv != nil:
			return "a method", false
		case isEntryPoint(decl):
			return "", true
		}
		return "a function", false
	}
	return "a declaration", false
}

// suiteVar classifies a var declaration. The blank assignment a suite hook
// is registered through is what the file is for; a spec registered the same way
// is the thing it must not hold, and is named as such because the two read
// alike.
func suiteVar(decl *ast.GenDecl) (what string, allowed bool) {
	hooks, specs := 0, 0
	for _, spec := range decl.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok || len(value.Names) != 1 || value.Names[0].Name != "_" || len(value.Values) != 1 {
			continue
		}
		call, ok := value.Values[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		switch name := callName(call.Fun); {
		case suiteHooks[name]:
			hooks++
		case specBuilders[name]:
			specs++
		}
	}

	switch {
	case specs > 0:
		return "a spec", false
	case hooks == len(decl.Specs) && hooks > 0:
		return "", true
	}
	return "a variable declaration", false
}

// isEntryPoint reports whether decl is a go test entry point a suite file
// exists to hold: func TestXxx(t *testing.T), or the func TestMain(m
// *testing.M) that wraps the whole binary.
func isEntryPoint(decl *ast.FuncDecl) bool {
	if !isTestName(decl.Name.Name, "Test") ||
		decl.Type.TypeParams != nil || decl.Type.Results != nil ||
		decl.Type.Params == nil || len(decl.Type.Params.List) != 1 {
		return false
	}

	pointer, ok := decl.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	qualified, ok := pointer.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return qualified.Sel.Name == "T" ||
		(decl.Name.Name == "TestMain" && qualified.Sel.Name == "M")
}
