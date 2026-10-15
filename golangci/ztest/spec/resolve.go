package spec

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"
)

// The packages a call is resolved against.
const (
	gomegaPath  = "github.com/onsi/gomega"
	assertPath  = "github.com/stretchr/testify/assert"
	requirePath = "github.com/stretchr/testify/require"
	errorsPath  = "errors"
	fmtPath     = "fmt"
	stringsPath = "strings"
)

// funcOf returns the function or method expr names, through any instantiation
// of a generic one, or nil when it names neither.
func (p *passChecker) funcOf(expr ast.Expr) *types.Func {
	var id *ast.Ident
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	case *ast.IndexExpr:
		return p.funcOf(e.X)
	case *ast.IndexListExpr:
		return p.funcOf(e.X)
	default:
		return nil
	}
	fn, _ := p.pass.TypesInfo.Uses[id].(*types.Func)
	return fn
}

// refers returns the name of the package-level function expr names when
// the package at path declares it.
func (p *passChecker) refers(expr ast.Expr, path string) (string, bool) {
	fn := p.funcOf(expr)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != path || fn.Signature().Recv() != nil {
		return "", false
	}
	return fn.Name(), true
}

// gomega reports whether fn is declared by Gomega: its root package or one
// beneath it, which is where the Gomega interface and WithT's methods live.
func gomega(fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	path := fn.Pkg().Path()
	return path == gomegaPath || strings.HasPrefix(path, gomegaPath+"/")
}

// testify reports whether fn is declared by testify's assert or require
// package, as a function or as a method of its Assertions.
func testify(fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	path := fn.Pkg().Path()
	return path == assertPath || path == requirePath
}

// variable is one variable the pass's files declare, with every write
// to it they hold.
type variable struct {
	// fn is the function declaring the variable, or nil for one at package
	// level.
	fn ast.Node

	// writes are the assignments to the variable, in source order.
	writes []write
}

// write is one assignment to a variable.
type write struct {
	// value is the expression assigned, or nil for a write that cannot
	// be followed: an operator assignment, an increment, a range clause.
	value ast.Expr

	// index is the result of value a multi-value assignment takes, or -1
	// when value is assigned whole.
	index int

	// fn is the function the assignment sits in, or nil for a package-level
	// declaration.
	fn ast.Node
}

// collectVariables finds every write the files make to the variables they
// declare. A write through a pointer or a field is not one.
func collectVariables(files []*ast.File, info *types.Info) map[*types.Var]*variable {
	c := &collector{info: info, vars: map[*types.Var]*variable{}}
	for _, file := range files {
		ast.PreorderStack(file, nil, c.visit)
	}
	return c.vars
}

type collector struct {
	info *types.Info
	vars map[*types.Var]*variable
}

func (c *collector) visit(n ast.Node, stack []ast.Node) bool {
	fn := enclosingFunc(stack)
	switch n := n.(type) {
	case *ast.Ident:
		if v, ok := c.info.Defs[n].(*types.Var); ok && !v.IsField() {
			c.variable(v).fn = fn
		}
	case *ast.ValueSpec:
		for i, name := range n.Names {
			switch {
			case len(n.Values) == len(n.Names):
				c.write(name, write{value: n.Values[i], index: -1, fn: fn})
			case len(n.Values) == 1:
				c.write(name, write{value: n.Values[0], index: i, fn: fn})
			}
		}
	case *ast.AssignStmt:
		c.assign(n, fn)
	case *ast.IncDecStmt:
		c.write(n.X, write{index: -1, fn: fn})
	case *ast.RangeStmt:
		for _, expr := range []ast.Expr{n.Key, n.Value} {
			if expr != nil {
				c.write(expr, write{index: -1, fn: fn})
			}
		}
	}
	return true
}

func (c *collector) assign(s *ast.AssignStmt, fn ast.Node) {
	if s.Tok != token.ASSIGN && s.Tok != token.DEFINE {
		for _, lhs := range s.Lhs {
			c.write(lhs, write{index: -1, fn: fn})
		}
		return
	}
	for i, lhs := range s.Lhs {
		switch {
		case len(s.Rhs) == len(s.Lhs):
			c.write(lhs, write{value: s.Rhs[i], index: -1, fn: fn})
		case len(s.Rhs) == 1:
			c.write(lhs, write{value: s.Rhs[0], index: i, fn: fn})
		}
	}
}

// write records w against the variable expr names, if it names one.
func (c *collector) write(expr ast.Expr, w write) {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return
	}
	obj := c.info.Defs[id]
	if obj == nil {
		obj = c.info.Uses[id]
	}
	if v, ok := obj.(*types.Var); ok && !v.IsField() {
		found := c.variable(v)
		found.writes = append(found.writes, w)
	}
}

func (c *collector) variable(v *types.Var) *variable {
	found := c.vars[v]
	if found == nil {
		found = &variable{}
		c.vars[v] = found
	}
	return found
}

// enclosingFunc returns the innermost function declaration or literal
// on stack, or nil at package level.
func enclosingFunc(stack []ast.Node) ast.Node {
	for _, n := range slices.Backward(stack) {
		switch n.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return n
		}
	}
	return nil
}
