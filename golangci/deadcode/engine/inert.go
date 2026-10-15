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
)

// subjectField is an exported field of a struct the module takes from its
// caller.
type subjectField struct {
	field *types.Var

	// owner is the name of the subject the field was first reached through,
	// which a promoted field shares with the struct embedding it.
	owner string

	// root is the API function or method whose callers set the field.
	root string
}

// polarity is the direction a value crosses the API: authored by the caller
// and taken by the module, or handed by the module to the caller.
type polarity bool

const (
	authored polarity = true
	handed   polarity = false
)

// inert is one inert-field run: the subjects the API's polarity reaches, then
// the reads live module code makes of them.
type inert struct {
	fset   *token.FileSet
	module map[string]bool
	live   reachability

	subjects map[string]*subjectField
	order    []string
	walked   typeutil.Map // types.Type → map[polarity]bool

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
		fset:     prog.Fset,
		module:   analyzedPackages(pkgs),
		live:     live,
		subjects: map[string]*subjectField{},
		echoes:   map[string]map[string]bool{},
		read:     map[string]bool{},
	}
	for _, name := range echoes {
		in.echoes[name] = map[string]bool{}
	}
	for _, root := range apiRoots(pkgs, surface) {
		in.signature(root.sig, authored, handed, root.name)
	}

	declared := in.reads(pkgs)
	in.conversions(prog)

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
			Detail: name + " is set by callers of " + subject.root +
				" and read by nothing; act on it or remove it",
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

// apiRoot is a function or method a caller of the API calls.
type apiRoot struct {
	name string
	sig  *types.Signature
}

// apiRoots returns the exported functions, and the exported methods of exported
// non-interface types, of the API packages, in name order. An interface's
// methods are not roots: whether its caller calls them or implements them
// is the polarity it is reached in.
func apiRoots(pkgs []*packages.Package, surface *apiSurface) []apiRoot {
	var roots []apiRoot
	for _, pkg := range pkgs {
		if pkg.Types == nil || testVariant(pkg) || !surface.packages[pkg.PkgPath] || !exportedRootPackage(pkg.PkgPath) {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			switch obj := scope.Lookup(name).(type) {
			case *types.Func:
				if obj.Exported() {
					roots = append(roots, apiRoot{name: pkg.PkgPath + "." + name, sig: obj.Signature()})
				}
			case *types.TypeName:
				named, ok := obj.Type().(*types.Named)
				if !ok || !obj.Exported() || obj.IsAlias() || types.IsInterface(named) {
					continue
				}
				for selection := range types.NewMethodSet(types.NewPointer(named)).Methods() {
					method, ok := selection.Obj().(*types.Func)
					if ok && method.Exported() {
						roots = append(roots, apiRoot{
							name: pkg.PkgPath + "." + name + "." + method.Name(),
							sig:  method.Signature(),
						})
					}
				}
			}
		}
	}
	slices.SortFunc(roots, func(x, y apiRoot) int { return strings.Compare(x.name, y.name) })
	return roots
}

// signature walks a signature's parameters in one polarity and its results
// in another.
func (in *inert) signature(sig *types.Signature, params, results polarity, root string) {
	for param := range sig.Params().Variables() {
		in.walk(param.Type(), params, root)
	}
	for result := range sig.Results().Variables() {
		in.walk(result.Type(), results, root)
	}
}

// walk follows a type the API carries in polarity p. It crosses pointers,
// slices, arrays, map keys and elements, channel elements, type arguments
// and a named non-struct type's underlying type; descends the exported
// and embedded fields of every struct; and flips polarity across a func type's
// parameters and an interface method's, which the other side supplies. It stays
// inside the module.
func (in *inert) walk(t types.Type, p polarity, root string) {
	t = types.Unalias(t)
	walked, _ := in.walked.At(t).(map[polarity]bool)
	if walked[p] {
		return
	}
	if walked == nil {
		walked = map[polarity]bool{}
		in.walked.Set(t, walked)
	}
	walked[p] = true

	switch t := t.(type) {
	case *types.Pointer:
		in.walk(t.Elem(), p, root)
	case *types.Slice:
		in.walk(t.Elem(), p, root)
	case *types.Array:
		in.walk(t.Elem(), p, root)
	case *types.Map:
		in.walk(t.Key(), p, root)
		in.walk(t.Elem(), p, root)
	case *types.Chan:
		in.walk(t.Elem(), p, root)
	case *types.Signature:
		in.signature(t, !p, p, root)
	case *types.Interface:
		in.methods(t, p, root)
	case *types.Struct:
		in.fields(t, p, root)
	case *types.Named:
		for arg := range t.TypeArgs().Types() {
			in.walk(arg, p, root)
		}
		if t.Obj().Pkg() == nil || !in.module[t.Obj().Pkg().Path()] {
			return
		}
		switch u := t.Underlying().(type) {
		case *types.Struct:
			if p == authored && t.Obj().Exported() {
				in.subject(t.Obj().Name(), u, root)
			}
			in.fields(u, p, root)
		case *types.Interface:
			in.methods(u, p, root)
		default:
			in.walk(u, p, root)
		}
	}
}

// methods walks an interface's exported methods: the other side of the API
// implements what this side calls.
func (in *inert) methods(iface *types.Interface, p polarity, root string) {
	for method := range iface.Methods() {
		if method.Exported() {
			in.signature(method.Signature(), !p, p, root)
		}
	}
}

// fields walks a struct's exported fields, and its embedded ones, whose
// exported fields are promoted.
func (in *inert) fields(st *types.Struct, p polarity, root string) {
	for field := range st.Fields() {
		if field.Exported() || field.Embedded() {
			in.walk(field.Type(), p, root)
		}
	}
}

// subject records a subject's exported fields, and those a struct it embeds
// by value promotes onto it. An embedded field is a container rather than
// a setting, and is not itself judged.
func (in *inert) subject(owner string, st *types.Struct, root string) {
	for field := range st.Fields() {
		if field.Embedded() {
			if inner, ok := embeddedStruct(field); ok {
				in.subject(owner, inner, root)
			}
			continue
		}
		if !field.Exported() || field.Pkg() == nil || !in.module[field.Pkg().Path()] {
			continue
		}
		key := declKey(position(in.fset, field.Pos()))
		if _, ok := in.subjects[key]; !ok {
			in.subjects[key] = &subjectField{field: field, owner: owner, root: root}
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

// conversions records, for each conversion to an interface in a live function
// outside test files, a read of every exported field beneath the converted
// value: reflection can read any of them, and fmt does.
func (in *inert) conversions(prog *program) {
	for fn := range prog.funcs {
		if !fn.Pos().IsValid() {
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
				if conversion, ok := instr.(*ssa.MakeInterface); ok {
					in.beneath(conversion.X.Type(), r, &typeutil.Map{})
				}
			}
		}
	}
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
		if t.Obj().Pkg() != nil && in.module[t.Obj().Pkg().Path()] {
			in.beneath(t.Underlying(), r, seen)
		}
	case *types.Struct:
		for field := range t.Fields() {
			if field.Exported() {
				in.record(field, r)
			}
			if field.Exported() || field.Embedded() {
				in.beneath(field.Type(), r, seen)
			}
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
