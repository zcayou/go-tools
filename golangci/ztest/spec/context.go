package spec

import (
	"go/ast"
	"go/types"
	"strings"
	"unicode"
	"unicode/utf8"
)

// backgroundContexts reports every reference in a test file
// to context.Background or context.TODO, outside the functions that run with no
// test handle to take a context from: a top-level Example function,
// and TestMain.
func (p *passChecker) backgroundContexts(file *ast.File) {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && handleless(fn.Name.Name) {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if name, ok := p.background(n.Sel); ok {
					p.reportBackground(n, name)
					return false
				}
			case *ast.Ident:
				if name, ok := p.background(n); ok {
					p.reportBackground(n, name)
				}
			}
			return true
		})
	}
}

// background reports whether id names context.Background or context.TODO,
// however the file imports the package, and which.
func (p *passChecker) background(id *ast.Ident) (string, bool) {
	fn, ok := p.pass.TypesInfo.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "context" {
		return "", false
	}
	switch fn.Name() {
	case "Background", "TODO":
		return "context." + fn.Name(), true
	default:
		return "", false
	}
}

func (p *passChecker) reportBackground(at ast.Node, name string) {
	p.report(at.Pos(), RuleBackgroundContext, name+" in a test; take the context the test framework hands "+
		"this node (t.Context, b.Context, f.Context, SpecContext), or context.WithoutCancel of it for work "+
		"that outlives the node")
}

// handleless reports whether a top-level function of this name runs with no
// test handle: an Example, which go test runs with none, and TestMain, whose
// *testing.M carries no context.
func handleless(name string) bool {
	if name == "TestMain" {
		return true
	}
	rest, ok := strings.CutPrefix(name, "Example")
	if !ok {
		return false
	}
	if rest == "" {
		return true
	}
	first, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(first)
}
