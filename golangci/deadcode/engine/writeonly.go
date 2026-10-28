package engine

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// audience is who code is written for, from the narrowest to the widest:
// production, and tests, whom a declared test-facing package is written
// for as much as a test file is. Evidence counts toward a field only from code
// whose audience is no wider than the field's.
type audience int

const (
	audienceProduction audience = iota + 1
	audienceTest
)

func (a audience) String() string {
	switch a {
	case audienceProduction:
		return "production"
	case audienceTest:
		return "test"
	default:
		return "audience(" + strconv.Itoa(int(a)) + ")"
	}
}

// writeOnlySubject is a field write-only-field judges.
type writeOnlySubject struct {
	field *types.Var

	// name is the field qualified by the struct type declaring it.
	name string

	// audience is the audience of the file declaring the field.
	audience audience

	// owner is the declaration key of the struct type's name.
	owner string
}

// writeOnly is one write-only-field run: the fields the module declares,
// the writes and reads its code makes of them, and the audience of each read.
type writeOnly struct {
	fset   *token.FileSet
	module map[string]bool

	// files holds the audience of every file whose code is the module's.
	files map[string]audience

	// variants holds the type-checked packages that exist only because of tests,
	// whose synthesized initializers are test code.
	variants map[*types.Package]bool

	// testFacing holds the declared test-facing packages.
	testFacing map[string]bool

	subjects map[string]*writeOnlySubject
	written  map[string]bool

	// read holds the narrowest audience any code reading a field is written for,
	// by declaration key.
	read map[string]audience

	// reached and compared hold the narrowest audience each type has been read
	// for, by reflection's reach and by value.
	reached, compared typeutil.Map
}

// newWriteOnlyFields reports each field of a named struct type that module
// code writes and no code of its audience reads. Live and dead code count
// alike. With an API declared, the exported fields of the struct types
// a consumer can hold are left to it.
func newWriteOnlyFields(
	prog *program, pkgs []*packages.Package, surface *apiSurface, testFacing map[string]bool,
	facts map[*packages.Package]fileFacts,
) []Finding {
	w := &writeOnly{
		fset:       prog.Fset,
		module:     analyzedPackages(pkgs),
		files:      map[string]audience{},
		variants:   map[*types.Package]bool{},
		testFacing: testFacing,
		subjects:   map[string]*writeOnlySubject{},
		written:    map[string]bool{},
		read:       map[string]audience{},
	}
	for _, pkg := range pkgs {
		if pkg.Types != nil && testVariant(pkg) {
			w.variants[pkg.Types] = true
		}
	}
	w.syntax(pkgs, facts)
	w.instructions(prog)

	var exposed map[string]bool
	if len(surface.patterns) > 0 {
		exposed = exposedStructs(w.fset, pkgs, surface, w.module)
	}

	var findings []Finding
	for key, subject := range w.subjects {
		if !w.written[key] || subject.field.Exported() && exposed[subject.owner] {
			continue
		}
		read, ok := w.read[key]
		if ok && read <= subject.audience {
			continue
		}
		detail := subject.name + " is written and never read; read it or remove it"
		if ok {
			detail = subject.name + " is written and read only by tests; read it in production code or remove it"
		}
		findings = append(findings, Finding{
			Pos:     position(w.fset, subject.field.Pos()),
			Rule:    RuleWriteOnlyField,
			Name:    subject.name,
			Package: subject.field.Pkg().Path(),
			Detail:  detail,
		})
	}
	slices.SortFunc(findings, func(x, y Finding) int {
		return cmp.Or(strings.Compare(x.Pos.Filename, y.Pos.Filename), cmp.Compare(x.Pos.Offset, y.Pos.Offset))
	})
	return findings
}

// exposedStructs returns the struct types a consumer of the API can hold,
// by the declaration key of their name: the exported types of the API packages,
// which a consumer names, and what the API's signatures and variables reach
// in either direction, together with what lies beneath them, the exported
// methods of each such type included.
func exposedStructs(
	fset *token.FileSet, pkgs []*packages.Package, surface *apiSurface, module map[string]bool,
) map[string]bool {
	api := &exposure{module: module, called: true}
	api.roots(pkgs, surface)
	for _, pkg := range pkgs {
		if !apiPackage(pkg, surface) {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			if obj, ok := scope.Lookup(name).(*types.TypeName); ok && obj.Exported() {
				api.walk(obj.Type(), handed, "")
			}
		}
	}
	return api.crossed(fset)
}

// crossed returns the module struct types the walk crosses in either polarity,
// by the declaration key of their name, an instantiation counting as its
// generic type.
func (e *exposure) crossed(fset *token.FileSet) map[string]bool {
	crossed := map[string]bool{}
	e.walked.Iterate(func(t types.Type, _ any) {
		named, ok := t.(*types.Named)
		if !ok {
			return
		}
		if _, ok := named.Underlying().(*types.Struct); ok {
			crossed[declKey(position(fset, named.Origin().Obj().Pos()))] = true
		}
	})
	return crossed
}

// fileAudience is the audience of a file of the package at path.
func (w *writeOnly) fileAudience(path, name string) audience {
	if testFile(name) || w.testFacing[path] {
		return audienceTest
	}
	return audienceProduction
}

// syntax records the subjects the module's files declare, and the writes
// and selections its code makes. A file a test variant re-checks counts once.
// A root program's code is production code, and declares no subject.
func (w *writeOnly) syntax(pkgs []*packages.Package, facts map[*packages.Package]fileFacts) {
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			name := position(w.fset, file.Pos()).Filename
			if _, ok := w.files[name]; ok {
				continue
			}
			code := w.fileAudience(pkg.PkgPath, name)
			w.files[name] = code
			if !synthesizedPackage(pkg.PkgPath) && !facts[pkg].generated[name] {
				w.declarations(file, pkg.TypesInfo, code)
			}
			w.scan(file, pkg.TypesInfo, code)
		}
	}
}

// declarations records the non-blank, non-embedded fields of every struct type
// the file names, function-local ones included. A field of an anonymous struct
// type is no subject: identical unnamed struct types assign to each other, so
// no one declaration owns it.
func (w *writeOnly) declarations(file *ast.File, info *types.Info, code audience) {
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || spec.Assign.IsValid() {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		owner := declKey(position(w.fset, spec.Name.Pos()))
		for _, field := range st.Fields.List {
			for _, ident := range field.Names {
				obj, ok := info.Defs[ident].(*types.Var)
				if !ok || ident.Name == "_" {
					continue
				}
				key := declKey(position(w.fset, ident.Pos()))
				w.subjects[key] = &writeOnlySubject{
					field: obj, name: spec.Name.Name + "." + ident.Name, audience: code, owner: owner,
				}
			}
		}
		return true
	})
}

// fieldKey is the declaration key of a field, an instantiated generic type's
// field keyed as its origin's.
func (w *writeOnly) fieldKey(field *types.Var) string {
	return declKey(position(w.fset, field.Origin().Pos()))
}

// write records a write of a field.
func (w *writeOnly) write(field *types.Var) {
	w.written[w.fieldKey(field)] = true
}

// reads records a read of a field by code of the given audience.
func (w *writeOnly) reads(field *types.Var, code audience) {
	key := w.fieldKey(field)
	if read, ok := w.read[key]; !ok || code < read {
		w.read[key] = code
	}
}

// atomicWrites are the methods of sync/atomic's types, and the prefixes
// of its functions, that write the value they are given rather than read it.
var atomicWrites = []string{"Store", "Swap", "Add", "And", "Or"}

// scan records the field writes and selections in a file. A selection is a read
// unless it is what an assignment, op-assignment, inc/dec or range = writes,
// the field appended to itself, an atomic-typed field that a writing method
// is called on, or a field whose address a writing sync/atomic function
// takes, where the call's result is discarded: Add, Swap, And and Or return
// a value the field held, and code using it reads the field. An element
// of a composite literal writes its field.
func (w *writeOnly) scan(file *ast.File, info *types.Info, code audience) {
	targets := map[*ast.SelectorExpr]bool{}
	discarded := map[*ast.CallExpr]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		discard(n, discarded)
		switch n := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				target(info, lhs, targets)
				if n.Tok == token.ASSIGN && len(n.Lhs) == len(n.Rhs) {
					if arg, ok := selfAppend(info, lhs, n.Rhs[i]); ok {
						target(info, arg, targets)
					}
				}
			}
		case *ast.IncDecStmt:
			target(info, n.X, targets)
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				target(info, n.Key, targets)
				target(info, n.Value, targets)
			}
		case *ast.CallExpr:
			if arg, ok := atomicWrite(info, n); ok && discarded[n] {
				target(info, arg, targets)
			}
		case *ast.CompositeLit:
			w.literal(info, n)
		case *ast.SelectorExpr:
			selection := info.Selections[n]
			if selection == nil || selection.Kind() != types.FieldVal {
				return true
			}
			field, ok := selection.Obj().(*types.Var)
			if !ok {
				return true
			}
			if targets[n] {
				w.write(field)
			} else {
				w.reads(field, code)
			}
		}
		return true
	})
}

// discard records the calls whose result a statement discards: one standing
// as a statement, one go or defer runs, and one assigned to the blank
// identifier.
func discard(n ast.Node, discarded map[*ast.CallExpr]bool) {
	var calls []ast.Expr
	switch n := n.(type) {
	case *ast.ExprStmt:
		calls = append(calls, n.X)
	case *ast.GoStmt:
		calls = append(calls, n.Call)
	case *ast.DeferStmt:
		calls = append(calls, n.Call)
	case *ast.AssignStmt:
		if len(n.Lhs) != len(n.Rhs) {
			return
		}
		for i, lhs := range n.Lhs {
			if blank, ok := lhs.(*ast.Ident); ok && blank.Name == "_" {
				calls = append(calls, n.Rhs[i])
			}
		}
	}
	for _, expr := range calls {
		if call, ok := ast.Unparen(expr).(*ast.CallExpr); ok {
			discarded[call] = true
		}
	}
}

// literal records the fields a struct composite literal's elements write.
func (w *writeOnly) literal(info *types.Info, lit *ast.CompositeLit) {
	t := info.TypeOf(lit)
	if t == nil {
		return
	}
	if pointer, ok := t.Underlying().(*types.Pointer); ok {
		t = pointer.Elem()
	}
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return
	}
	for i, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok {
				if field, ok := info.Uses[key].(*types.Var); ok {
					w.write(field)
				}
			}
			continue
		}
		if i < st.NumFields() {
			w.write(st.Field(i))
		}
	}
}

// selfAppend returns the first argument of rhs when rhs appends to the field
// selection lhs assigns.
func selfAppend(info *types.Info, lhs, rhs ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(rhs).(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return nil, false
	}
	fun, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return nil, false
	}
	if builtin, ok := info.Uses[fun].(*types.Builtin); !ok || builtin.Name() != "append" {
		return nil, false
	}
	return call.Args[0], sameSelection(info, lhs, call.Args[0])
}

// sameSelection reports whether two expressions select the same field
// through the same variables.
func sameSelection(info *types.Info, x, y ast.Expr) bool {
	switch x := ast.Unparen(x).(type) {
	case *ast.Ident:
		y, ok := ast.Unparen(y).(*ast.Ident)
		return ok && info.Uses[x] != nil && info.Uses[x] == info.Uses[y]
	case *ast.StarExpr:
		y, ok := ast.Unparen(y).(*ast.StarExpr)
		return ok && sameSelection(info, x.X, y.X)
	case *ast.SelectorExpr:
		y, ok := ast.Unparen(y).(*ast.SelectorExpr)
		if !ok {
			return false
		}
		sx, sy := info.Selections[x], info.Selections[y]
		if sx == nil && sy == nil {
			// A qualified identifier: a package's variable.
			return info.Uses[x.Sel] != nil && info.Uses[x.Sel] == info.Uses[y.Sel]
		}
		return sx != nil && sy != nil && sx.Obj() == sy.Obj() && sameSelection(info, x.X, y.X)
	}
	return false
}

// atomicWrite returns the field selection a sync/atomic call writes: the atomic
// typed field a Store, Swap, Add, And or Or method is called on, or the field
// whose address a function of those names is given. Its caller decides whether
// the call only writes it.
func atomicWrite(info *types.Info, call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	fn, ok := info.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "sync/atomic" {
		return nil, false
	}
	if fn.Signature().Recv() != nil {
		if !slices.Contains(atomicWrites, fn.Name()) {
			return nil, false
		}
		recv, isField := ast.Unparen(sel.X).(*ast.SelectorExpr)
		if !isField || !atomicType(info.TypeOf(recv)) {
			return nil, false
		}
		return recv, true
	}
	if len(call.Args) == 0 || !slices.ContainsFunc(atomicWrites, func(prefix string) bool {
		return strings.HasPrefix(fn.Name(), prefix)
	}) {
		return nil, false
	}
	addr, ok := ast.Unparen(call.Args[0]).(*ast.UnaryExpr)
	if !ok || addr.Op != token.AND {
		return nil, false
	}
	return addr.X, true
}

// atomicType reports whether t is one of sync/atomic's types, held by value.
func atomicType(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "sync/atomic"
}

// instructions records the reads the module's built instructions make: what
// a conversion to an interface lets reflection reach, what equality,
// a conversion of a struct and a comparable type argument compare, and what
// reflect.TypeFor describes. A generic body is read in each instantiation,
// where its types are known. A dependency's body is not read.
func (w *writeOnly) instructions(prog *program) {
	for fn := range prog.funcs {
		initializer := fn.Synthetic == "package initializer"
		if fn.Synthetic != "" && fn.Syntax() == nil && !initializer {
			continue
		}
		code, ok := w.files[position(w.fset, fn.Pos()).Filename]
		if !ok && !initializer {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if initializer {
					if code, ok = w.initializerAudience(fn, instr); !ok {
						continue
					}
				}
				w.instruction(instr, code)
			}
		}
	}
}

// initializerAudience is the audience of an instruction of a package
// initializer: that of the file it is written in, or where SSA places
// the instruction nowhere, that of the package, a test variant's being test
// code.
func (w *writeOnly) initializerAudience(fn *ssa.Function, instr ssa.Instruction) (audience, bool) {
	if pos := instr.Pos(); pos.IsValid() {
		code, ok := w.files[position(w.fset, pos).Filename]
		return code, ok
	}
	path := fn.Pkg.Pkg.Path()
	switch {
	case !w.module[path]:
		return 0, false
	case w.variants[fn.Pkg.Pkg], w.testFacing[path]:
		return audienceTest, true
	default:
		return audienceProduction, true
	}
}

// instruction records the reads one instruction makes.
func (w *writeOnly) instruction(instr ssa.Instruction, code audience) {
	switch instr := instr.(type) {
	case *ssa.MakeInterface:
		w.boxed(instr, code)
	case *ssa.BinOp:
		if instr.Op == token.EQL || instr.Op == token.NEQ {
			w.byValue(instr.X.Type(), code)
		}
	case *ssa.ChangeType:
		if t := pointee(instr.X.Type()); isStruct(t) {
			w.byValue(t, code)
		}
	case *ssa.Convert:
		if basic, ok := types.Unalias(instr.Type()).(*types.Basic); ok && basic.Kind() == types.UnsafePointer {
			if pointer, ok := instr.X.Type().Underlying().(*types.Pointer); ok {
				w.byValue(pointer.Elem(), code)
			}
		}
	case *ssa.MakeMap:
		w.mapKey(instr.Type(), code)
	case *ssa.MapUpdate:
		w.mapKey(instr.Map.Type(), code)
	case *ssa.Lookup:
		w.mapKey(instr.X.Type(), code)
	}
	if call, ok := instr.(ssa.CallInstruction); ok {
		common := call.Common()
		if builtin, ok := common.Value.(*ssa.Builtin); ok && builtin.Name() == "delete" {
			w.mapKey(common.Args[0].Type(), code)
		}
	}
	for _, operand := range instr.Operands(nil) {
		if fn, ok := (*operand).(*ssa.Function); ok {
			w.instance(fn, code)
		}
	}
}

// boxed records what a conversion to an interface reads: what reflection
// reaches beneath the value, unless every use of the interface is the receiver
// of a method called through it or an argument of a standard-library sink.
// A method called hands the method the concrete value, whose reads its body
// counts; a sink that only holds it reads nothing, and one that compares
// it reads every field equality does.
func (w *writeOnly) boxed(mi *ssa.MakeInterface, code audience) {
	refs := *mi.Referrers()
	if len(refs) == 0 {
		w.reach(mi.X.Type(), code)
		return
	}
	compares := false
	for _, ref := range refs {
		call, ok := ref.(ssa.CallInstruction)
		if !ok {
			w.reach(mi.X.Type(), code)
			return
		}
		if invokes(call.Common(), mi) {
			continue
		}
		switch wholemodule.SinkOf(call.Common(), mi) {
		case wholemodule.SinkHolds:
		case wholemodule.SinkCompares:
			compares = true
		default:
			w.reach(mi.X.Type(), code)
			return
		}
	}
	if compares {
		w.byValue(mi.X.Type(), code)
	}
}

// invokes reports whether a call only calls a method of v: v is the receiver
// of an interface method call, and none of its arguments.
func invokes(call *ssa.CallCommon, v ssa.Value) bool {
	return call.IsInvoke() && call.Value == v && !slices.Contains(call.Args, v)
}

// instance records what an instantiated function reads of its type arguments:
// equality over every one a comparable constraint admits, and reflection's
// reach beneath the one reflect.TypeFor describes.
func (w *writeOnly) instance(fn *ssa.Function, code audience) {
	origin := fn.Origin()
	if origin == nil {
		return
	}
	args := fn.TypeArgs()
	if obj, ok := origin.Object().(*types.Func); ok && obj.Pkg() != nil &&
		obj.Pkg().Path() == "reflect" && obj.Name() == "TypeFor" && len(args) > 0 {
		w.reach(args[0], code)
	}
	params := origin.TypeParams()
	for i, arg := range args {
		if i >= params.Len() {
			break
		}
		if iface, ok := params.At(i).Constraint().Underlying().(*types.Interface); ok && iface.IsComparable() {
			w.byValue(arg, code)
		}
	}
}

// mapKey records equality over the key type of a map type.
func (w *writeOnly) mapKey(t types.Type, code audience) {
	if m, ok := t.Underlying().(*types.Map); ok {
		w.byValue(m.Key(), code)
	}
}

// pointee is the type a pointer type points to, or t itself.
func pointee(t types.Type) types.Type {
	if pointer, ok := t.Underlying().(*types.Pointer); ok {
		return pointer.Elem()
	}
	return t
}

func isStruct(t types.Type) bool {
	_, ok := t.Underlying().(*types.Struct)
	return ok
}

// visit reports whether a type still has to be read for code of the given
// audience, and marks it read for it. The memo holds types as types.Identical
// judges them, and two named struct types declaring the same fields have
// identical underlying types though their fields are distinct variables,
// so a named struct's fields are read through the named type rather than
// its underlying one.
func visit(seen *typeutil.Map, t types.Type, code audience) bool {
	if prior, ok := seen.At(t).(audience); ok && prior <= code {
		return false
	}
	seen.Set(t, code)
	return true
}

// reach records a read of every field a value of type t holds as reflection
// reaches them: through pointers, slices, arrays, maps and channels, into every
// field of every struct, exported or not, since fmt prints unexported fields
// and reflect.DeepEqual compares them. It does not descend a func type.
func (w *writeOnly) reach(t types.Type, code audience) {
	t = types.Unalias(t)
	if !visit(&w.reached, t, code) {
		return
	}
	switch t := t.(type) {
	case *types.Pointer:
		w.reach(t.Elem(), code)
	case *types.Slice:
		w.reach(t.Elem(), code)
	case *types.Array:
		w.reach(t.Elem(), code)
	case *types.Map:
		w.reach(t.Key(), code)
		w.reach(t.Elem(), code)
	case *types.Chan:
		w.reach(t.Elem(), code)
	case *types.Named:
		if st, ok := t.Underlying().(*types.Struct); ok {
			w.reachFields(st, code)
		} else {
			w.reach(t.Underlying(), code)
		}
	case *types.Struct:
		w.reachFields(t, code)
	}
}

// reachFields records a read of every field of a struct and of what reflection
// reaches beneath each.
func (w *writeOnly) reachFields(st *types.Struct, code audience) {
	for field := range st.Fields() {
		w.reads(field, code)
		w.reach(field.Type(), code)
	}
}

// byValue records a read of every field a value of type t holds by value,
// as equality compares them: the fields of a struct and of the structs
// and arrays it holds, and no further than a pointer.
func (w *writeOnly) byValue(t types.Type, code audience) {
	t = types.Unalias(t)
	if !visit(&w.compared, t, code) {
		return
	}
	switch t := t.(type) {
	case *types.Array:
		w.byValue(t.Elem(), code)
	case *types.Named:
		if st, ok := t.Underlying().(*types.Struct); ok {
			w.byValueFields(st, code)
		} else {
			w.byValue(t.Underlying(), code)
		}
	case *types.Struct:
		w.byValueFields(t, code)
	}
}

// byValueFields records a read of every field of a struct and of what
// equality compares beneath each.
func (w *writeOnly) byValueFields(st *types.Struct, code audience) {
	for field := range st.Fields() {
		w.reads(field, code)
		w.byValue(field.Type(), code)
	}
}
