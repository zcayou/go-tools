package engine

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// subjectField is an exported field of a struct the module takes from its
// caller.
type subjectField struct {
	field *types.Var

	// owner is the name of the subject the field was first reached through,
	// which a promoted field shares with the struct embedding it.
	owner string

	// setter says how callers set the field: by calling an API function or method,
	// or through an API variable.
	setter string
}

// polarity is the direction a value crosses the API: authored by the caller
// and taken by the module, or handed by the module to the caller.
type polarity bool

const (
	authored polarity = true
	handed   polarity = false
)

// exposure is the API's polarity walk: every type it crosses, in each polarity
// it crosses it.
type exposure struct {
	module map[string]bool
	walked typeutil.Map // types.Type → map[polarity]bool

	// authored, when set, is handed each exported module struct type the walk
	// reaches in authored polarity.
	authored func(owner string, st *types.Struct, setter string)

	// called also walks the exported methods of every module named type the walk
	// crosses that is not an interface, as a caller calls them.
	called bool
}

// inert is one inert-field run: the subjects the API's polarity reaches, then
// the reads live module code makes of them.
type inert struct {
	exposure

	fset *token.FileSet
	live reachability

	subjects map[string]*subjectField
	order    []string

	// echoes are the echo-functions entries, by name, and what each reads.
	echoes map[string]map[string]bool

	// read holds the subject fields some live module code reads, by declaration
	// key.
	read map[string]bool
}

// newInertFields reports each exported field of a struct the module takes
// from its caller that no live module code reads. live is what production
// roots reach.
func newInertFields(
	prog *program, pkgs []*packages.Package, surface *apiSurface, live reachability, echoes []string,
) ([]Finding, error) {
	in := &inert{
		module:   analyzedPackages(pkgs),
		fset:     prog.Fset,
		live:     live,
		subjects: map[string]*subjectField{},
		echoes:   map[string]map[string]bool{},
		read:     map[string]bool{},
	}
	in.authored = in.subject
	for _, name := range echoes {
		in.echoes[name] = map[string]bool{}
	}
	in.roots(pkgs, surface)

	declared := in.reads(pkgs)
	in.outflows(prog)

	deprecated := deprecatedFields(in.fset, pkgs)
	var findings []Finding
	reported := map[string]bool{}
	for _, key := range in.order {
		subject := in.subjects[key]
		if in.read[key] || deprecated[key] {
			continue
		}
		reported[key] = true
		name := subject.owner + "." + subject.field.Name()
		findings = append(findings, Finding{
			Pos:     position(in.fset, subject.field.Pos()),
			Rule:    RuleInertField,
			Name:    name,
			Package: subject.field.Pkg().Path(),
			Detail:  name + " is set " + subject.setter + " and read by nothing; act on it or remove it",
		})
	}

	for _, name := range echoes {
		if !declared[name] {
			return nil, &ConfigError{Setting: SettingEchoFunctions, Problem: ProblemNamesNoFunction, Value: name}
		}
		changes := false
		for key := range in.echoes[name] {
			changes = changes || reported[key]
		}
		if !changes {
			return nil, &ConfigError{Setting: SettingEchoFunctions, Problem: ProblemChangesNothing, Value: name}
		}
	}
	return findings, nil
}

// apiRoot is what a caller of the API reaches: a function or method it calls,
// or a variable it assigns and reads.
type apiRoot struct {
	name string

	// setter says how a caller sets what the root takes, as a finding words it.
	setter string

	// takes are the types the caller hands the module through the root, and gives
	// the types the module hands back.
	takes, gives []types.Type
}

// funcRoot is a function or method a caller calls: it takes the parameters
// and gives the results.
func funcRoot(name string, sig *types.Signature) apiRoot {
	root := apiRoot{name: name, setter: "by callers of " + name}
	for param := range sig.Params().Variables() {
		root.takes = append(root.takes, param.Type())
	}
	for result := range sig.Results().Variables() {
		root.gives = append(root.gives, result.Type())
	}
	return root
}

// apiRoots returns the exported functions and variables, and the exported
// methods of exported non-interface types, of the API packages, in name order.
// A caller both assigns a variable and reads it. An interface's methods are not
// roots: whether its caller calls them or implements them is the polarity
// it is reached in.
func apiRoots(pkgs []*packages.Package, surface *apiSurface) []apiRoot {
	var roots []apiRoot
	for _, pkg := range pkgs {
		if !apiPackage(pkg, surface) {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			switch obj := scope.Lookup(name).(type) {
			case *types.Func:
				if obj.Exported() {
					roots = append(roots, funcRoot(pkg.PkgPath+"."+name, obj.Signature()))
				}
			case *types.Var:
				if obj.Exported() {
					qualified := pkg.PkgPath + "." + name
					roots = append(roots, apiRoot{
						name: qualified, setter: "through " + qualified,
						takes: []types.Type{obj.Type()}, gives: []types.Type{obj.Type()},
					})
				}
			case *types.TypeName:
				named, ok := obj.Type().(*types.Named)
				if !ok || !obj.Exported() || obj.IsAlias() || types.IsInterface(named) {
					continue
				}
				for selection := range types.NewMethodSet(types.NewPointer(named)).Methods() {
					method, ok := selection.Obj().(*types.Func)
					if ok && method.Exported() {
						roots = append(roots, funcRoot(pkg.PkgPath+"."+name+"."+method.Name(), method.Signature()))
					}
				}
			}
		}
	}
	slices.SortFunc(roots, func(x, y apiRoot) int { return strings.Compare(x.name, y.name) })
	return roots
}

// roots walks what every root of the API takes from its caller and gives back.
func (e *exposure) roots(pkgs []*packages.Package, surface *apiSurface) {
	for _, root := range apiRoots(pkgs, surface) {
		for _, t := range root.takes {
			e.walk(t, authored, root.setter)
		}
		for _, t := range root.gives {
			e.walk(t, handed, root.setter)
		}
	}
}

// apiPackage reports whether a loaded package is one of the API's own, rather
// than a test variant of one.
func apiPackage(pkg *packages.Package, surface *apiSurface) bool {
	return pkg.Types != nil && !testVariant(pkg) && surface.packages[pkg.PkgPath] && exportedRootPackage(pkg.PkgPath)
}

// signature walks a signature's parameters in one polarity and its results
// in another.
func (e *exposure) signature(sig *types.Signature, params, results polarity, setter string) {
	for param := range sig.Params().Variables() {
		e.walk(param.Type(), params, setter)
	}
	for result := range sig.Results().Variables() {
		e.walk(result.Type(), results, setter)
	}
}

// walk follows a type the API carries in polarity p. It crosses pointers,
// slices, arrays, map keys and elements, channel elements, type arguments
// and a named non-struct type's underlying type; descends the exported
// and embedded fields of every struct; and flips polarity across a func type's
// parameters and an interface method's, which the other side supplies. It stays
// inside the module.
func (e *exposure) walk(t types.Type, p polarity, setter string) {
	t = types.Unalias(t)
	walked, _ := e.walked.At(t).(map[polarity]bool)
	if walked[p] {
		return
	}
	if walked == nil {
		walked = map[polarity]bool{}
		e.walked.Set(t, walked)
	}
	walked[p] = true

	switch t := t.(type) {
	case *types.Pointer:
		e.walk(t.Elem(), p, setter)
	case *types.Slice:
		e.walk(t.Elem(), p, setter)
	case *types.Array:
		e.walk(t.Elem(), p, setter)
	case *types.Map:
		e.walk(t.Key(), p, setter)
		e.walk(t.Elem(), p, setter)
	case *types.Chan:
		e.walk(t.Elem(), p, setter)
	case *types.Signature:
		e.signature(t, !p, p, setter)
	case *types.Interface:
		e.methods(t, p, setter)
	case *types.Struct:
		e.fields(t, p, setter)
	case *types.Named:
		e.named(t, p, setter)
	}
}

// named walks a named type's type arguments and, for a module type, what
// it is: a struct's fields, an interface's methods, or another type's
// underlying type, and under called the signatures of its exported methods.
// An exported struct met in authored polarity is a subject.
func (e *exposure) named(t *types.Named, p polarity, setter string) {
	for arg := range t.TypeArgs().Types() {
		e.walk(arg, p, setter)
	}
	if t.Obj().Pkg() == nil || !e.module[t.Obj().Pkg().Path()] {
		return
	}
	if e.called && !types.IsInterface(t) {
		for selection := range types.NewMethodSet(types.NewPointer(t)).Methods() {
			if method, ok := selection.Obj().(*types.Func); ok && method.Exported() {
				e.signature(method.Signature(), authored, handed, setter)
			}
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Struct:
		if p == authored && t.Obj().Exported() && e.authored != nil {
			e.authored(t.Obj().Name(), u, setter)
		}
		e.fields(u, p, setter)
	case *types.Interface:
		e.methods(u, p, setter)
	default:
		e.walk(u, p, setter)
	}
}

// methods walks an interface's exported methods: the other side of the API
// implements what this side calls.
func (e *exposure) methods(iface *types.Interface, p polarity, setter string) {
	for method := range iface.Methods() {
		if method.Exported() {
			e.signature(method.Signature(), !p, p, setter)
		}
	}
}

// fields walks a struct's exported fields, and its embedded ones, whose
// exported fields are promoted.
func (e *exposure) fields(st *types.Struct, p polarity, setter string) {
	for field := range st.Fields() {
		if field.Exported() || field.Embedded() {
			e.walk(field.Type(), p, setter)
		}
	}
}

// subject records a subject's exported fields, and those a struct it embeds
// by value promotes onto it. An embedded field is a container rather than
// a setting, and is not itself judged.
func (in *inert) subject(owner string, st *types.Struct, setter string) {
	for field := range st.Fields() {
		if field.Embedded() {
			if inner, ok := embeddedStruct(field); ok {
				in.subject(owner, inner, setter)
			}
			continue
		}
		if !field.Exported() || field.Pkg() == nil || !in.module[field.Pkg().Path()] {
			continue
		}
		key := declKey(position(in.fset, field.Pos()))
		if _, ok := in.subjects[key]; !ok {
			in.subjects[key] = &subjectField{field: field, owner: owner, setter: setter}
			in.order = append(in.order, key)
		}
	}
}

// embeddedStruct returns the struct a field embeds by value.
func embeddedStruct(field *types.Var) (*types.Struct, bool) {
	st, ok := types.Unalias(field.Type()).Underlying().(*types.Struct)
	return st, ok && field.Embedded()
}

// reader is how a function's reads count: toward acting on a field, toward
// an echo-functions entry, or not at all.
type reader struct {
	counts bool
	echo   string
}

// readerOf classifies a top-level function. A String, GoString or Format method
// echoes its fields into text, and so does what echo-functions lists.
func (in *inert) readerOf(fn *types.Func) reader {
	if fn == nil {
		return reader{counts: true}
	}
	if fn.Signature().Recv() != nil && slices.Contains([]string{"String", "GoString", "Format"}, fn.Name()) {
		return reader{}
	}
	if name := funcName(fn); in.echoes[name] != nil {
		return reader{echo: name}
	}
	return reader{counts: true}
}

// funcName writes a function as echo-functions does: import/path.Func,
// import/path.Type.Method or import/path.(*Type).Method.
func funcName(fn *types.Func) string {
	pkg := fn.Pkg().Path()
	recv := fn.Signature().Recv()
	if recv == nil {
		return pkg + "." + fn.Name()
	}
	t := recv.Type()
	pointer, ok := t.(*types.Pointer)
	if ok {
		t = pointer.Elem()
	}
	named, isNamed := types.Unalias(t).(*types.Named)
	if !isNamed {
		return pkg + "." + fn.Name()
	}
	if ok {
		return pkg + ".(*" + named.Obj().Name() + ")." + fn.Name()
	}
	return pkg + "." + named.Obj().Name() + "." + fn.Name()
}

// record notes a read of a field by r.
func (in *inert) record(field *types.Var, r reader) {
	key := declKey(position(in.fset, field.Pos()))
	switch {
	case r.counts:
		in.read[key] = true
	case r.echo != "":
		in.echoes[r.echo][key] = true
	}
}

// reads records every field selection module code makes outside test files,
// and returns the names of the module's functions, as echo-functions writes
// them.
func (in *inert) reads(pkgs []*packages.Package) map[string]bool {
	declared := map[string]bool{}
	scanned := map[string]bool{}
	for _, pkg := range pkgs {
		if synthesizedPackage(pkg.PkgPath) {
			continue
		}
		for _, file := range pkg.Syntax {
			name := position(in.fset, file.Pos()).Filename
			if testFile(name) || scanned[name] {
				continue
			}
			scanned[name] = true
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok {
					// A package-level initializer runs whenever its package is loaded.
					in.scan(decl, pkg.TypesInfo, true, reader{counts: true})
					continue
				}
				fn, _ := pkg.TypesInfo.Defs[fd.Name].(*types.Func)
				if fn != nil {
					declared[funcName(fn)] = true
				}
				if fd.Body != nil {
					live := in.live.all[declKey(position(in.fset, fd.Name.Pos()))]
					in.scan(fd.Body, pkg.TypesInfo, live, in.readerOf(fn))
				}
			}
		}
	}
	return declared
}

// scan records the field reads in node, made in a function that is live or not.
// A field selection is a read unless it is what an assignment writes. Reading
// a whole embedded value reads every field it promotes; selecting through one
// does not.
func (in *inert) scan(node ast.Node, info *types.Info, live bool, r reader) {
	targets := map[*ast.SelectorExpr]bool{}
	through := map[*ast.SelectorExpr]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			in.scan(n.Body, info, in.live.all[declKey(position(in.fset, n.Type.Func))], r)
			return false
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				target(info, lhs, targets)
			}
		case *ast.IncDecStmt:
			target(info, n.X, targets)
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				target(info, n.Key, targets)
				target(info, n.Value, targets)
			}
		case *ast.SelectorExpr:
			selection := info.Selections[n]
			if selection == nil {
				return true
			}
			if inner, ok := ast.Unparen(n.X).(*ast.SelectorExpr); ok {
				through[inner] = true
			}
			field, ok := selection.Obj().(*types.Var)
			if !ok || selection.Kind() != types.FieldVal || targets[n] || !live {
				return true
			}
			in.record(field, r)
			if inner, ok := embeddedStruct(field); ok && !through[n] {
				in.promoted(inner, r)
			}
		}
		return true
	})
}

// promoted records a read of every field a struct embedded by value promotes.
func (in *inert) promoted(st *types.Struct, r reader) {
	for field := range st.Fields() {
		in.record(field, r)
		if inner, ok := embeddedStruct(field); ok {
			in.promoted(inner, r)
		}
	}
}

// target marks the field selections an assignment's left-hand side writes:
// the selection itself, and the struct-valued fields it selects through.
func target(info *types.Info, expr ast.Expr, targets map[*ast.SelectorExpr]bool) {
	sel, ok := ast.Unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return
	}
	selection := info.Selections[sel]
	if selection == nil || selection.Kind() != types.FieldVal {
		return
	}
	targets[sel] = true
	if _, value := info.TypeOf(sel.X).Underlying().(*types.Struct); value {
		target(info, sel.X, targets)
	}
}

// outflows records a read of every exported field beneath a value live module
// code outside test files lets out of its sight: converting it to an interface,
// since reflection can read any of them and fmt does, or handing it to consumer
// code, which can read any of them too. A value is handed over as an argument
// of a method selected on an interface a consumer can implement, as a call,
// a method value or a method expression, and as an argument of a call through
// a func value of a type the caller supplies through the API. A generic body
// is judged in each instantiation, where its types are known.
func (in *inert) outflows(prog *program) {
	for fn := range prog.funcs {
		if !fn.Pos().IsValid() || fn.Synthetic != "" && fn.Syntax() == nil {
			continue
		}
		pos := position(in.fset, fn.Pos())
		if testFile(pos.Filename) || !in.live.all[declKey(pos)] {
			continue
		}
		top := fn
		for top.Parent() != nil {
			top = top.Parent()
		}
		if origin := top.Origin(); origin != nil {
			top = origin
		}
		obj, _ := top.Object().(*types.Func)
		r := in.readerOf(obj)
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				in.outflow(instr, r)
			}
		}
	}
}

// outflow records the reads one instruction makes by letting values out.
func (in *inert) outflow(instr ssa.Instruction, r reader) {
	switch instr := instr.(type) {
	case *ssa.MakeInterface:
		in.beneath(instr.X.Type(), r, &typeutil.Map{})
	case *ssa.MakeClosure:
		// A bound method value: the closure's binding is the receiver.
		bound, ok := instr.Fn.(*ssa.Function)
		if ok && interfaceMethod(bound) != nil && len(instr.Bindings) == 1 && in.implementable(instr.Bindings[0].Type()) {
			in.handOff(bound.Signature.Params(), 0, r)
		}
	}
	if call, ok := instr.(ssa.CallInstruction); ok {
		common := call.Common()
		switch {
		case common.IsInvoke():
			if in.implementable(common.Value.Type()) {
				in.handOff(common.Signature().Params(), 0, r)
			}
		case common.StaticCallee() == nil:
			if t := common.Value.Type(); in.supplied(t) || in.supplied(t.Underlying()) {
				in.handOff(common.Signature().Params(), 0, r)
			}
		}
	}
	for _, operand := range instr.Operands(nil) {
		// A method expression's thunk takes the receiver as its first parameter,
		// which is the interface itself unless the method is promoted from one
		// a struct embeds.
		thunk, ok := (*operand).(*ssa.Function)
		if !ok || len(thunk.FreeVars) > 0 || thunk.Signature.Params().Len() == 0 {
			continue
		}
		method := interfaceMethod(thunk)
		if method == nil {
			continue
		}
		params := thunk.Signature.Params()
		recv := params.At(0).Type()
		if !types.IsInterface(recv) {
			recv = method.Signature().Recv().Type()
		}
		if in.implementable(recv) {
			in.handOff(params, 1, r)
		}
	}
}

// interfaceMethod returns the interface method a synthesized thunk or bound
// method wrapper calls, or nil when fn is none.
func interfaceMethod(fn *ssa.Function) *types.Func {
	if fn.Synthetic == "" || fn.Syntax() != nil || fn.Signature.Recv() != nil {
		return nil
	}
	method, ok := fn.Object().(*types.Func)
	if !ok || method.Signature().Recv() == nil || !types.IsInterface(method.Signature().Recv().Type()) {
		return nil
	}
	return method
}

// handOff records a read of every exported field beneath the parameters from
// the first'th on.
func (in *inert) handOff(params *types.Tuple, first int, r reader) {
	for i := first; i < params.Len(); i++ {
		in.beneath(params.At(i).Type(), r, &typeutil.Map{})
	}
}

// implementable reports whether code outside the module can implement
// an interface type: one declared outside it, one open as zerrors uses
// the word, an interface written in place that carries no unexported method,
// or one the API takes from its caller, which a consumer satisfies structurally
// whatever its declaration.
func (in *inert) implementable(t types.Type) bool {
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		if !types.IsInterface(t) {
			return false
		}
		if t.Obj().Pkg() == nil || !in.module[t.Obj().Pkg().Path()] || !wholemodule.Closed(t) {
			return true
		}
	case *types.Interface:
		if wholemodule.Implementable(t) {
			return true
		}
	default:
		return false
	}
	return in.supplied(t)
}

// supplied reports whether the polarity walk crosses t in authored polarity:
// the caller supplies a value of it.
func (e *exposure) supplied(t types.Type) bool {
	walked, _ := e.walked.At(types.Unalias(t)).(map[polarity]bool)
	return walked[authored]
}

// beneath records a read of every exported field a value of type t holds,
// as reflection reaches them, inside the module.
func (in *inert) beneath(t types.Type, r reader, seen *typeutil.Map) {
	t = types.Unalias(t)
	if seen.At(t) != nil {
		return
	}
	seen.Set(t, true)
	switch t := t.(type) {
	case *types.Pointer:
		in.beneath(t.Elem(), r, seen)
	case *types.Slice:
		in.beneath(t.Elem(), r, seen)
	case *types.Array:
		in.beneath(t.Elem(), r, seen)
	case *types.Map:
		in.beneath(t.Key(), r, seen)
		in.beneath(t.Elem(), r, seen)
	case *types.Named:
		if t.Obj().Pkg() == nil || !in.module[t.Obj().Pkg().Path()] {
			return
		}
		// Identical struct types are one entry of seen, though each named struct
		// declares fields of its own.
		if st, ok := t.Underlying().(*types.Struct); ok {
			in.beneathFields(st, r, seen)
		} else {
			in.beneath(t.Underlying(), r, seen)
		}
	case *types.Struct:
		in.beneathFields(t, r, seen)
	}
}

// beneathFields records a read of a struct's exported fields, and of what
// reflection reaches beneath them and its embedded ones.
func (in *inert) beneathFields(st *types.Struct, r reader, seen *typeutil.Map) {
	for field := range st.Fields() {
		if field.Exported() {
			in.record(field, r)
		}
		if field.Exported() || field.Embedded() {
			in.beneath(field.Type(), r, seen)
		}
	}
}

// deprecatedFields returns the struct fields, by declaration key, whose doc
// carries a Deprecated: paragraph. Keeping a field that does nothing, so
// callers still compile, is what deprecation means.
func deprecatedFields(fset *token.FileSet, pkgs []*packages.Package) map[string]bool {
	deprecated := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				st, ok := n.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range st.Fields.List {
					if !deprecation(field.Doc) && !deprecation(field.Comment) {
						continue
					}
					for _, name := range field.Names {
						deprecated[declKey(position(fset, name.Pos()))] = true
					}
				}
				return true
			})
		}
	}
	return deprecated
}

// deprecation reports whether a comment holds a paragraph opening
// with Deprecated:, as go doc reads one.
func deprecation(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for paragraph := range strings.SplitSeq(doc.Text(), "\n\n") {
		if strings.HasPrefix(paragraph, "Deprecated:") {
			return true
		}
	}
	return false
}
