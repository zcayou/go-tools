package engine

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
)

// printer is how a function formats its operands: printf kind takes a format
// string and formats the operands after it by its verbs, and print kind
// formats every operand as %v.
type printer struct {
	// format is the index of a printf printer's format parameter, or -1
	// for a print printer, whose operands start at its final, variadic parameter.
	format int
}

// printf reports whether p takes a format.
func (p printer) printf() bool {
	return p.format >= 0
}

// known are the printers vet's printf analyzer knows by name. A name ending
// in f is printf kind.
var known = map[string]bool{
	"fmt.Append": true, "fmt.Appendf": true, "fmt.Appendln": true, "fmt.Errorf": true,
	"fmt.Fprint": true, "fmt.Fprintf": true, "fmt.Fprintln": true,
	"fmt.Print": true, "fmt.Printf": true, "fmt.Println": true,
	"fmt.Sprint": true, "fmt.Sprintf": true, "fmt.Sprintln": true,

	"log.Fatal": true, "log.Fatalf": true, "log.Fatalln": true,
	"log.Panic": true, "log.Panicf": true, "log.Panicln": true,
	"log.Print": true, "log.Printf": true, "log.Println": true,
	"(*log.Logger).Fatal": true, "(*log.Logger).Fatalf": true, "(*log.Logger).Fatalln": true,
	"(*log.Logger).Panic": true, "(*log.Logger).Panicf": true, "(*log.Logger).Panicln": true,
	"(*log.Logger).Print": true, "(*log.Logger).Printf": true, "(*log.Logger).Println": true,

	"(*testing.common).Error": true, "(*testing.common).Errorf": true,
	"(*testing.common).Fatal": true, "(*testing.common).Fatalf": true,
	"(*testing.common).Log": true, "(*testing.common).Logf": true,
	"(*testing.common).Skip": true, "(*testing.common).Skipf": true,
	"(testing.TB).Error": true, "(testing.TB).Errorf": true,
	"(testing.TB).Fatal": true, "(testing.TB).Fatalf": true,
	"(testing.TB).Log": true, "(testing.TB).Logf": true,
	"(testing.TB).Skip": true, "(testing.TB).Skipf": true,
}

// printerOf returns how fn formats its operands, if it is a printer: one vet
// knows by name; a module function, method or interface method inferred
// to be one; or a foreign function or method whose body the load does not hold,
// when its name ends in f and its parameters end in a format string and ...any.
func (a *analysis) printerOf(fn *types.Func) (printer, bool) {
	fn = fn.Origin()
	if p, ok := a.printers[fn]; ok {
		return p, true
	}
	if known[fn.FullName()] {
		return shaped(fn, strings.HasSuffix(fn.Name(), "f"))
	}
	if fn.Pkg() == nil || a.modulePackage(fn.Pkg()) {
		return printer{}, false
	}
	if !strings.HasSuffix(fn.Name(), "f") {
		return printer{}, false
	}
	return shaped(fn, true)
}

// shaped returns the printer fn's signature makes it: printf kind when it ends
// in a format string and ...any, and print kind when it ends in ...any.
func shaped(fn *types.Func, printf bool) (printer, bool) {
	sig := fn.Signature()
	if printf {
		at, ok := formatParam(sig)
		return printer{format: at}, ok
	}
	return printer{format: -1}, variadicAny(sig)
}

// variadicAny reports whether sig's final parameter is ...any.
func variadicAny(sig *types.Signature) bool {
	params := sig.Params()
	if !sig.Variadic() || params.Len() == 0 {
		return false
	}
	slice, ok := params.At(params.Len() - 1).Type().(*types.Slice)
	if !ok {
		return false
	}
	iface, ok := slice.Elem().Underlying().(*types.Interface)
	return ok && iface.Empty()
}

// inferPrinters finds the module's printers, as vet's printf analyzer finds
// the wrappers of the ones it knows, to a least fixed point: a function
// or method whose final parameter is ...any, and whose body forwards it, last
// and spread, to a printer, without assigning to it or taking its address.
// It is printf kind when it also forwards the string parameter before
// it as a printf printer's format, and print kind otherwise. A method
// of a module interface is a printer when every module type converted
// to the interface implements it with a printer of one kind.
func (a *analysis) inferPrinters(functions []*ssa.Function) {
	a.printers = map[*types.Func]printer{}
	var candidates []forwarder
	for _, pkg := range a.module.Packages {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				fn, ok := pkg.TypesInfo.Defs[fd.Name].(*types.Func)
				if !ok || !variadicAny(fn.Signature()) {
					continue
				}
				candidates = append(candidates, forwarder{fn: fn, decl: fd, info: pkg.TypesInfo})
			}
		}
	}
	implementers := a.converted(functions)

	for changed := true; changed; {
		changed = false
		for _, c := range candidates {
			if _, done := a.printers[c.fn]; done {
				continue
			}
			if p, ok := c.forwards(a); ok {
				a.printers[c.fn] = p
				changed = true
			}
		}
		for method, converted := range implementers {
			if _, done := a.printers[method]; done {
				continue
			}
			if p, ok := a.implementedBy(method, converted); ok {
				a.printers[method] = p
				changed = true
			}
		}
	}
}

// forwarder is a module function or method that may forward its operands
// to a printer.
type forwarder struct {
	fn   *types.Func
	decl *ast.FuncDecl
	info *types.Info
}

// forwards reports the printer f is, given the printers found so far.
func (f forwarder) forwards(a *analysis) (printer, bool) {
	sig := f.fn.Signature()
	params := sig.Params()
	operands := params.At(params.Len() - 1)
	var format *types.Var
	if at, ok := formatParam(sig); ok {
		format = params.At(at)
	}

	var found *printer
	tainted := false
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			tainted = tainted || slices.ContainsFunc(n.Lhs, func(lhs ast.Expr) bool { return f.refers(lhs, operands) })
		case *ast.UnaryExpr:
			tainted = tainted || n.Op == token.AND && f.refers(n.X, operands)
		case *ast.CallExpr:
			if found != nil || !n.Ellipsis.IsValid() || !f.refers(n.Args[len(n.Args)-1], operands) {
				return true
			}
			callee, ok := typeutil.Callee(f.info, n).(*types.Func)
			if !ok {
				return true
			}
			p, ok := a.printerOf(callee)
			switch {
			case !ok:
			case !p.printf():
				found = &printer{format: -1}
			case format != nil && len(n.Args) >= 2 && f.refers(n.Args[len(n.Args)-2], format):
				found = &printer{format: params.Len() - 2}
			}
			// A printf printer handed some other format prints nothing the caller wrote
			// as a format.
		}
		return true
	})
	if tainted || found == nil {
		return printer{}, false
	}
	return *found, true
}

// refers reports whether expr is a use of v.
func (f forwarder) refers(expr ast.Expr, v *types.Var) bool {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && f.info.Uses[id] == v
}

// converted returns, for each method of a module interface, the module types
// converted to that interface.
func (a *analysis) converted(functions []*ssa.Function) map[*types.Func][]types.Type {
	found := map[*types.Func][]types.Type{}
	seen := map[*types.Func]*typeutil.Map{}
	for _, fn := range functions {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				conversion, ok := instr.(*ssa.MakeInterface)
				if !ok {
					continue
				}
				named, ok := types.Unalias(conversion.Type()).(*types.Named)
				if !ok || named.Obj().Pkg() == nil || !a.modulePackage(named.Obj().Pkg()) {
					continue
				}
				iface, ok := named.Underlying().(*types.Interface)
				if !ok {
					continue
				}
				for method := range iface.Methods() {
					if seen[method] == nil {
						seen[method] = &typeutil.Map{}
					}
					if seen[method].At(conversion.X.Type()) == nil {
						seen[method].Set(conversion.X.Type(), true)
						found[method] = append(found[method], conversion.X.Type())
					}
				}
			}
		}
	}
	return found
}

// implementedBy reports the printer an interface method is when every type
// converted to its interface implements it with a printer of one kind.
func (a *analysis) implementedBy(method *types.Func, implementers []types.Type) (printer, bool) {
	at, printf := formatParam(method.Signature())
	if !printf && !variadicAny(method.Signature()) {
		return printer{}, false
	}
	want := printer{format: -1}
	if printf {
		want = printer{format: at}
	}
	for _, t := range implementers {
		obj, _, _ := types.LookupFieldOrMethod(t, true, method.Pkg(), method.Name())
		concrete, ok := obj.(*types.Func)
		if !ok {
			return printer{}, false
		}
		p, ok := a.printerOf(concrete)
		if !ok || p.printf() != want.printf() {
			return printer{}, false
		}
	}
	return want, len(implementers) > 0
}

// modulePackage reports whether pkg is one the module declares.
func (a *analysis) modulePackage(pkg *types.Package) bool {
	return a.modulePaths[pkg.Path()]
}
