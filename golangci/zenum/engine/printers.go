package engine

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
	"golang.org/x/tools/refactor/satisfy"
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

// printerOf returns how obj formats its operands, if it is a printer:
//   - a function or method vet knows by name;
//   - a module function, method, interface method, variable or field inferred
//     to be one;
//   - a foreign function or method whose body the load does not hold, when its
//     name ends in f and its parameters end in a format string and ...any;
//   - or a func value of that shape, a variable, field or parameter, whose name
//     or whose type's name ends in f, whatever it holds.
//
// A test package's copy of a module declaration is that declaration.
func (c *corpus) printerOf(obj types.Object) (printer, bool) {
	switch obj := obj.(type) {
	case *types.Func:
		fn := obj.Origin()
		if p, ok := c.printers[c.a.canonical(fn)]; ok {
			return p, true
		}
		if known[fn.FullName()] {
			return shaped(fn.Signature(), strings.HasSuffix(fn.Name(), "f"))
		}
		if fn.Pkg() == nil || c.a.modulePackage(fn.Pkg()) || !strings.HasSuffix(fn.Name(), "f") {
			return printer{}, false
		}
		return shaped(fn.Signature(), true)
	case *types.Var:
		v := obj.Origin()
		if p, ok := c.printers[c.a.canonical(v)]; ok {
			return p, true
		}
		sig := funcSignature(v.Type())
		if sig == nil || !namedPrintf(v) {
			return printer{}, false
		}
		return shaped(sig, true)
	}
	return printer{}, false
}

// signatureOf returns the signature a call through obj has: a function's
// or method's own, or the type of a func value.
func signatureOf(obj types.Object) *types.Signature {
	if fn, ok := obj.(*types.Func); ok {
		return fn.Signature()
	}
	return funcSignature(obj.Type())
}

// funcSignature returns the signature t is a func type of, or nil.
func funcSignature(t types.Type) *types.Signature {
	sig, _ := types.Unalias(t).Underlying().(*types.Signature)
	return sig
}

// namedPrintf reports whether a func value is named for a printf printer:
// its name, or the name of the type it is declared with, ends in f.
func namedPrintf(v *types.Var) bool {
	if strings.HasSuffix(v.Name(), "f") {
		return true
	}
	declared, ok := v.Type().(interface{ Obj() *types.TypeName })
	return ok && strings.HasSuffix(declared.Obj().Name(), "f")
}

// shaped returns the printer a signature makes: printf kind when it ends
// in a format string and ...any, and print kind when it ends in ...any.
func shaped(sig *types.Signature, printf bool) (printer, bool) {
	if printf {
		at, ok := formatParam(sig)
		return printer{format: at}, ok
	}
	return printer{format: -1}, variadicAny(sig)
}

// widest returns the printer a func value or interface method of signature
// sig is when it may run any of the printers found: printf kind when one
// of them is and sig takes a format, and print kind otherwise.
func widest(sig *types.Signature, found []printer) (printer, bool) {
	if len(found) == 0 {
		return printer{}, false
	}
	if slices.ContainsFunc(found, printer.printf) {
		if at, ok := formatParam(sig); ok {
			return printer{format: at}, true
		}
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

// inferPrinters finds the printers the corpus's code sees, as vet's printf
// analyzer finds the wrappers of the ones it knows, to a least fixed point:
//   - A function or method whose final parameter is ...any, and whose body
//     forwards it, last and spread, to a printer, without assigning
//     to it or taking its address. It is printf kind when it also forwards
//     the string parameter before it as a printf printer's format, and print
//     kind otherwise.
//   - A variable or field of such a func type that module code assigns
//     a printer: a function literal forwarding as such a function does,
//     a function or method value naming a printer, or another printer variable
//     or field, so that copies carry it.
//   - A method of a module interface some implementation of which is a printer:
//     a type the interface is satisfied by, as refactor/satisfy finds them
//     in the module's syntax and as MakeInterface converts to it, which covers
//     the instantiations.
//
// A variable, field or interface method that may run a printf printer is printf
// kind, and one whose values or implementations are not all printers,
// as a no-op default is not, is a printer still: which runs decides whether
// the line is printed, not what it would say.
//
// The corpus of test files infers from the module's files and its own,
// so a printer a test file declares is one to the test files reaching it,
// and to no plain file.
func (c *corpus) inferPrinters() {
	c.printers = map[types.Object]printer{}
	candidates := c.candidates()
	assigned := c.assignments()
	implementers := c.implementations()

	for changed := true; changed; {
		changed = false
		widen := func(obj types.Object, p printer) {
			obj = c.a.canonical(obj)
			if old, ok := c.printers[obj]; !ok || !old.printf() && p.printf() {
				c.printers[obj] = p
				changed = true
			}
		}
		for _, candidate := range candidates {
			if _, done := c.printers[c.a.canonical(candidate.fn)]; done {
				continue
			}
			if p, ok := candidate.forwards(c); ok {
				widen(candidate.fn, p)
			}
		}
		for v, values := range assigned {
			if p, ok := widest(funcSignature(v.Type()), c.valuePrinters(values)); ok {
				widen(v, p)
			}
		}
		for method, implementing := range implementers {
			if p, ok := widest(method.Signature(), c.implementedBy(implementing)); ok {
				widen(method, p)
			}
		}
	}
}

// candidates returns the functions and methods the corpus's sources declare
// whose final parameter is ...any.
func (c *corpus) candidates() []candidate {
	var found []candidate
	for _, src := range c.sources {
		pkg := src.pkg
		for _, file := range src.files {
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				fn, ok := pkg.TypesInfo.Defs[fd.Name].(*types.Func)
				if !ok || !variadicAny(fn.Signature()) {
					continue
				}
				found = append(found, candidate{fn: fn, sig: fn.Signature(), body: fd.Body, info: pkg.TypesInfo})
			}
		}
	}
	return found
}

// candidate is a module function or method that may forward its operands
// to a printer.
type candidate struct {
	fn *types.Func
	forwarder
}

// forwarder is a body that may forward its operands to a printer: a function's
// or method's, or a function literal's.
type forwarder struct {
	sig  *types.Signature
	body *ast.BlockStmt
	info *types.Info
}

// forwards reports the printer f is, given the printers found so far.
func (f forwarder) forwards(c *corpus) (printer, bool) {
	params := f.sig.Params()
	operands := params.At(params.Len() - 1)
	var format *types.Var
	if at, ok := formatParam(f.sig); ok {
		format = params.At(at)
	}

	var found *printer
	tainted := false
	ast.Inspect(f.body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			tainted = tainted || slices.ContainsFunc(n.Lhs, func(lhs ast.Expr) bool { return f.refers(lhs, operands) })
		case *ast.UnaryExpr:
			tainted = tainted || n.Op == token.AND && f.refers(n.X, operands)
		case *ast.CallExpr:
			if found != nil || !n.Ellipsis.IsValid() || !f.refers(n.Args[len(n.Args)-1], operands) {
				return true
			}
			callee := typeutil.Callee(f.info, n)
			if callee == nil {
				return true
			}
			p, ok := c.printerOf(callee)
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

// assignedValue is a value module code assigns to a variable or field,
// with the type information of the package writing it.
type assignedValue struct {
	expr ast.Expr
	info *types.Info
}

// assignments returns, for each variable and field of a func type whose final
// parameter is ...any, the values the corpus's sources assign it: a var spec's
// value, an assignment statement's, or a struct literal's element.
func (c *corpus) assignments() map[*types.Var][]assignedValue {
	found := map[*types.Var][]assignedValue{}
	for _, src := range c.sources {
		info := src.pkg.TypesInfo
		record := func(v *types.Var, value ast.Expr) {
			if v == nil || v.Name() == "_" {
				return
			}
			if sig := funcSignature(v.Type()); sig != nil && variadicAny(sig) {
				found[v.Origin()] = append(found[v.Origin()], assignedValue{expr: value, info: info})
			}
		}
		for _, file := range src.files {
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.ValueSpec:
					if len(n.Names) == len(n.Values) {
						for i, name := range n.Names {
							v, _ := info.Defs[name].(*types.Var)
							record(v, n.Values[i])
						}
					}
				case *ast.AssignStmt:
					if len(n.Lhs) == len(n.Rhs) && (n.Tok == token.ASSIGN || n.Tok == token.DEFINE) {
						for i, lhs := range n.Lhs {
							record(assignedVar(info, lhs), n.Rhs[i])
						}
					}
				case *ast.CompositeLit:
					elements(info, n, record)
				}
				return true
			})
		}
	}
	return found
}

// elements calls record with each field a struct literal sets and the value
// it sets it to, keyed or in order.
func elements(info *types.Info, lit *ast.CompositeLit, record func(*types.Var, ast.Expr)) {
	st, ok := types.Unalias(info.TypeOf(lit)).Underlying().(*types.Struct)
	if !ok {
		return
	}
	for i, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok {
				field, _ := info.Uses[key].(*types.Var)
				record(field, kv.Value)
			}
		} else if i < st.NumFields() {
			record(st.Field(i), elt)
		}
	}
}

// assignedVar returns the variable or field an assignment's left-hand side
// writes, or nil.
func assignedVar(info *types.Info, lhs ast.Expr) *types.Var {
	switch lhs := ast.Unparen(lhs).(type) {
	case *ast.Ident:
		v, _ := info.ObjectOf(lhs).(*types.Var)
		return v
	case *ast.SelectorExpr:
		if selection := info.Selections[lhs]; selection != nil {
			if selection.Kind() != types.FieldVal {
				return nil
			}
			v, _ := selection.Obj().(*types.Var)
			return v
		}
		v, _ := info.Uses[lhs.Sel].(*types.Var)
		return v
	}
	return nil
}

// valuePrinters returns the printers among the values assigned to a variable
// or field.
func (c *corpus) valuePrinters(values []assignedValue) []printer {
	var found []printer
	for _, value := range values {
		if p, ok := c.valuePrinter(value); ok {
			found = append(found, p)
		}
	}
	return found
}

// valuePrinter returns the printer an assigned value is: a function literal
// forwarding as a module printer does, or a function, method value, variable
// or field naming a printer.
func (c *corpus) valuePrinter(value assignedValue) (printer, bool) {
	switch e := ast.Unparen(value.expr).(type) {
	case *ast.FuncLit:
		sig, ok := value.info.TypeOf(e).(*types.Signature)
		if !ok || !variadicAny(sig) {
			return printer{}, false
		}
		return forwarder{sig: sig, body: e.Body, info: value.info}.forwards(c)
	case *ast.Ident:
		return c.printerOf(value.info.Uses[e])
	case *ast.SelectorExpr:
		if selection := value.info.Selections[e]; selection != nil {
			if selection.Kind() == types.MethodExpr {
				return printer{}, false
			}
			return c.printerOf(selection.Obj())
		}
		return c.printerOf(value.info.Uses[e.Sel])
	}
	return printer{}, false
}

// implementations returns, for each method of a module interface whose final
// parameter is ...any, the methods implementing it on the types the interface
// is satisfied by: those refactor/satisfy finds in the corpus's sources,
// and those a MakeInterface the corpus infers from converts to it.
func (c *corpus) implementations() map[*types.Func][]*types.Func {
	found := map[*types.Func][]*types.Func{}
	seen := map[*types.Func]*typeutil.Map{}
	add := func(iface, concrete types.Type) {
		it, ok := iface.Underlying().(*types.Interface)
		if !ok {
			return
		}
		for method := range it.Methods() {
			method = method.Origin()
			if method.Pkg() == nil || !c.a.modulePackage(method.Pkg()) || !variadicAny(method.Signature()) {
				continue
			}
			if seen[method] == nil {
				seen[method] = &typeutil.Map{}
			}
			if seen[method].At(concrete) != nil {
				continue
			}
			seen[method].Set(concrete, true)
			obj, _, _ := types.LookupFieldOrMethod(concrete, true, method.Pkg(), method.Name())
			if implementing, ok := obj.(*types.Func); ok && implementing.Origin() != method {
				found[method] = append(found[method], implementing)
			}
		}
	}
	var finder satisfy.Finder
	for _, src := range c.sources {
		finder.Find(src.pkg.TypesInfo, src.files)
	}
	for constraint := range finder.Result {
		add(constraint.LHS, constraint.RHS)
	}
	for _, fn := range c.inferred {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if conversion, ok := instr.(*ssa.MakeInterface); ok {
					add(conversion.Type(), conversion.X.Type())
				}
			}
		}
	}
	return found
}

// implementedBy returns the printers among the methods implementing
// an interface's method.
func (c *corpus) implementedBy(implementing []*types.Func) []printer {
	var found []printer
	for _, method := range implementing {
		if p, ok := c.printerOf(method); ok {
			found = append(found, p)
		}
	}
	return found
}

// modulePackage reports whether pkg is one the module declares.
func (a *analysis) modulePackage(pkg *types.Package) bool {
	return a.modulePaths[pkg.Path()]
}
