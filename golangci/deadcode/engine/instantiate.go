package engine

import (
	"go/ast"
	"go/types"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// maxResolvedVectors bounds how many concrete vectors one generic object may
// hold. The fixpoint terminates on its own — Go rejects instantiation cycles,
// and loadErrors fails the run before a program carrying one gets here — so
// the cap is not what makes resolution finish; it bounds what a pathological
// program can make every credit pass that reads the vectors pay. The excess
// is dropped after a sort, so a capped object yields the same vectors on every
// run, and dropping degrades toward crediting less, never toward crediting
// wrongly.
const maxResolvedVectors = 1024

// instantiations resolves every generic function and generic type name
// in the loaded program to the fully concrete type-argument vectors the program
// instantiates it with. TypesInfo.Instances records vectors as written,
// and inside a generic body they mention the enclosing declaration's type
// parameters — evidence that is real but not yet usable, because no concrete
// method set matches a free parameter. Such a vector is re-emitted under every
// concrete vector of its enclosing generic, transitively to a fixpoint, so
// an instantiation several generics deep still resolves to the arguments
// the program runs it with.
type instantiations struct {
	resolved map[types.Object][][]types.Type
}

// enclosingGeneric is the innermost generic declaration around an instantiation
// site: the object whose concrete vectors stand in for the site's free type
// parameters, and the parameter objects a parametric vector references,
// in declaration order. Inside a method of a generic type the two halves
// deliberately differ — vectors are recorded against the named type, while
// a parametric vector references the method's own receiver type parameters,
// which are distinct objects from the type's; an environment keyed
// on the type's own parameters would never match and silently resolve nothing.
type enclosingGeneric struct {
	obj    types.Object
	params []*types.TypeParam
}

// rawInstance is one instantiation site as recorded: the generic object,
// the argument vector as written, and the enclosing generic when the site sits
// inside one.
type rawInstance struct {
	obj       types.Object
	args      []types.Type
	enclosing *enclosingGeneric
}

// newInstantiations reads TypesInfo.Instances from every loaded package,
// dependencies included, and resolves parametric vectors transitively. Sites
// in files the view does not admit contribute nothing, which masks
// constraint-satisfaction credits, reflect.TypeFor closure seeds,
// and instantiation-derived flow edges in one place.
func newInstantiations(loaded []*packages.Package, v view) *instantiations {
	var raw []rawInstance
	packages.Visit(loaded, nil, func(pkg *packages.Package) {
		if pkg.TypesInfo == nil || !v.admitsPackage(pkg.PkgPath) {
			return
		}
		for _, file := range pkg.Syntax {
			if !v.admitsFile(position(pkg.Fset, file.Pos()).Filename) {
				continue
			}
			raw = append(raw, fileInstances(pkg, file)...)
		}
	})
	return resolveInstances(raw)
}

// vectors yields every fully concrete type-argument vector recorded
// for a generic function or generic type name, deduplicated and in a stable
// order. A generic object never instantiated with concrete arguments yields
// nothing.
func (s *instantiations) vectors(obj types.Object) [][]types.Type {
	if canonical := canonicalGeneric(obj); canonical != nil {
		obj = canonical
	}
	return s.resolved[obj]
}

// all yields every generic object holding at least one resolved vector,
// with its vectors.
func (s *instantiations) all(yield func(obj types.Object, vectors [][]types.Type) bool) {
	for obj, vectors := range s.resolved {
		if !yield(obj, vectors) {
			return
		}
	}
}

// fileInstances collects the file's instantiation sites, each with its
// enclosing generic. The enclosing declaration is decided at the top level
// because that is the only level Go permits one: a local type cannot be generic
// and a function literal has no type parameters of its own, so a site's free
// parameters always belong to the declaration that holds it.
func fileInstances(pkg *packages.Package, file *ast.File) []rawInstance {
	var raw []rawInstance
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			raw = append(raw, nodeInstances(pkg, decl, funcEnclosing(pkg, decl))...)
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				var enclosing *enclosingGeneric
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					enclosing = typeSpecEnclosing(pkg, typeSpec)
				}
				raw = append(raw, nodeInstances(pkg, spec, enclosing)...)
			}
		}
	}
	return raw
}

func nodeInstances(pkg *packages.Package, node ast.Node, enclosing *enclosingGeneric) []rawInstance {
	var raw []rawInstance
	ast.Inspect(node, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		instance, ok := pkg.TypesInfo.Instances[ident]
		if !ok || instance.TypeArgs.Len() == 0 {
			return true
		}
		obj := canonicalGeneric(pkg.TypesInfo.Uses[ident])
		if obj == nil {
			return true
		}
		raw = append(raw, rawInstance{obj: obj, args: slices.Collect(instance.TypeArgs.Types()), enclosing: enclosing})
		return true
	})
	return raw
}

// funcEnclosing returns the generic context a function declaration opens:
// the function's own type parameters when it is generic, the receiver's when
// it is a method of a generic type, nil for everything else.
func funcEnclosing(pkg *packages.Package, decl *ast.FuncDecl) *enclosingGeneric {
	fn, ok := pkg.TypesInfo.Defs[decl.Name].(*types.Func)
	if !ok {
		return nil
	}
	sig := fn.Signature()
	if sig.TypeParams().Len() > 0 {
		return &enclosingGeneric{obj: fn, params: slices.Collect(sig.TypeParams().TypeParams())}
	}
	if sig.RecvTypeParams().Len() == 0 {
		return nil
	}
	recv := types.Unalias(sig.Recv().Type())
	if pointer, isPointer := recv.(*types.Pointer); isPointer {
		recv = types.Unalias(pointer.Elem())
	}
	named, ok := recv.(*types.Named)
	if !ok {
		return nil
	}
	return &enclosingGeneric{obj: named.Origin().Obj(), params: slices.Collect(sig.RecvTypeParams().TypeParams())}
}

func typeSpecEnclosing(pkg *packages.Package, spec *ast.TypeSpec) *enclosingGeneric {
	name, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName)
	if !ok {
		return nil
	}
	named, ok := name.Type().(*types.Named)
	if !ok || named.TypeParams().Len() == 0 {
		return nil
	}
	return &enclosingGeneric{obj: name, params: slices.Collect(named.TypeParams().TypeParams())}
}

// canonicalGeneric maps a reference to a generic function or generic type name
// onto the origin object instantiations are indexed by, and anything else
// to nil. Instances hands back origin objects already; normalizing here as well
// lets a caller ask about whichever form it holds.
func canonicalGeneric(obj types.Object) types.Object {
	switch obj := obj.(type) {
	case *types.Func:
		if origin := obj.Origin(); origin.Signature().TypeParams().Len() > 0 {
			return origin
		}
	case *types.TypeName:
		if named, ok := types.Unalias(obj.Type()).(*types.Named); ok && named.TypeParams().Len() > 0 {
			return named.Origin().Obj()
		}
	}
	return nil
}

// resolveInstances splits the recorded sites into concrete vectors
// and parametric ones, then re-emits each parametric vector under every
// concrete vector of its enclosing generic until nothing new appears.
// A parametric site with no enclosing generic cannot occur in code that type
// checks, and is dropped rather than guessed at; so is a substitution
// that fails. Both degrade toward resolving fewer vectors, which downstream
// means crediting less, never crediting wrongly.
func resolveInstances(raw []rawInstance) *instantiations {
	sets := map[types.Object]map[string][]types.Type{}
	add := func(obj types.Object, vector []types.Type) bool {
		set := sets[obj]
		if set == nil {
			set = map[string][]types.Type{}
			sets[obj] = set
		}
		key := vectorKey(vector)
		if _, ok := set[key]; ok {
			return false
		}
		set[key] = vector
		return true
	}

	type parametricSite struct {
		rawInstance
		substituted map[string]bool
	}
	var parametric []parametricSite
	for _, site := range raw {
		if !slices.ContainsFunc(site.args, parametricType) {
			add(site.obj, site.args)
		} else if site.enclosing != nil {
			parametric = append(parametric, parametricSite{rawInstance: site, substituted: map[string]bool{}})
		}
	}

	for changed := true; changed; {
		changed = false
		for _, site := range parametric {
			for _, key := range slices.Sorted(maps.Keys(sets[site.enclosing.obj])) {
				vector := sets[site.enclosing.obj][key]
				if site.substituted[key] || len(vector) != len(site.enclosing.params) {
					continue
				}
				site.substituted[key] = true
				args, ok := substituteVector(site.args, environment(site.enclosing.params, vector))
				if ok && add(site.obj, args) {
					changed = true
				}
			}
		}
	}

	resolved := make(map[types.Object][][]types.Type, len(sets))
	for obj, set := range sets {
		keys := slices.Sorted(maps.Keys(set))
		keys = keys[:min(len(keys), maxResolvedVectors)]
		vectors := make([][]types.Type, len(keys))
		for i, key := range keys {
			vectors[i] = set[key]
		}
		resolved[obj] = vectors
	}
	return &instantiations{resolved: resolved}
}

// environment binds each type parameter to the same-index element of a concrete
// vector.
func environment(params []*types.TypeParam, vector []types.Type) map[*types.TypeParam]types.Type {
	env := make(map[*types.TypeParam]types.Type, len(params))
	for i, param := range params {
		env[param] = vector[i]
	}
	return env
}

func substituteVector(args []types.Type, env map[*types.TypeParam]types.Type) ([]types.Type, bool) {
	vector := make([]types.Type, len(args))
	for i, arg := range args {
		substituted, ok := substitute(arg, env)
		if !ok {
			return nil, false
		}
		vector[i] = substituted
	}
	return vector, true
}

// vectorKey identifies a vector by its fully qualified type strings, which
// is both the deduplication key and the order capping applies under.
func vectorKey(vector []types.Type) string {
	parts := make([]string, len(vector))
	for i, typ := range vector {
		parts[i] = types.TypeString(typ, nil)
	}
	return strings.Join(parts, ",")
}

// substitute rewrites t with each type parameter replaced by its binding
// in env. A type mentioning no type parameter is returned unchanged regardless
// of shape, so a shape substitution cannot rebuild only ever fails where
// substitution is genuinely required. A free type parameter absent from env,
// or a parametric shape outside the handled list — union terms among them —
// reports false rather than guessing.
func substitute(t types.Type, env map[*types.TypeParam]types.Type) (types.Type, bool) {
	if !parametricType(t) {
		return t, true
	}
	switch t := types.Unalias(t).(type) {
	case *types.TypeParam:
		bound, ok := env[t]
		return bound, ok
	case *types.Named:
		return substituteNamed(t, env)
	case *types.Pointer:
		if elem, ok := substitute(t.Elem(), env); ok {
			return types.NewPointer(elem), true
		}
	case *types.Slice:
		if elem, ok := substitute(t.Elem(), env); ok {
			return types.NewSlice(elem), true
		}
	case *types.Array:
		if elem, ok := substitute(t.Elem(), env); ok {
			return types.NewArray(elem, t.Len()), true
		}
	case *types.Chan:
		if elem, ok := substitute(t.Elem(), env); ok {
			return types.NewChan(t.Dir(), elem), true
		}
	case *types.Map:
		return substituteMap(t, env)
	case *types.Signature:
		if sig, ok := substituteSignature(t, env); ok {
			return sig, true
		}
	case *types.Struct:
		return substituteStruct(t, env)
	case *types.Interface:
		return substituteInterface(t, env)
	}
	return nil, false
}

// substituteNamed re-instantiates a named type under env. Instantiation runs
// on the origin with the substituted arguments and validation off: the compiler
// already verified the site this vector descends from, and re-checking
// constraints here could only reject what it accepted. A named type
// that is parametric yet carries no type arguments is a bare generic name,
// which no value can have as its type.
func substituteNamed(named *types.Named, env map[*types.TypeParam]types.Type) (types.Type, bool) {
	if named.TypeArgs().Len() == 0 {
		return nil, false
	}
	args := make([]types.Type, 0, named.TypeArgs().Len())
	for arg := range named.TypeArgs().Types() {
		substituted, ok := substitute(arg, env)
		if !ok {
			return nil, false
		}
		args = append(args, substituted)
	}
	instantiated, err := types.Instantiate(nil, named.Origin(), args, false)
	if err != nil {
		return nil, false
	}
	return instantiated, true
}

func substituteMap(m *types.Map, env map[*types.TypeParam]types.Type) (types.Type, bool) {
	key, ok := substitute(m.Key(), env)
	if !ok {
		return nil, false
	}
	elem, ok := substitute(m.Elem(), env)
	if !ok {
		return nil, false
	}
	return types.NewMap(key, elem), true
}

// substituteSignature rebuilds a signature over substituted parameter
// and result tuples. The receiver is dropped: the one place a receiver-bearing
// signature reaches here is an interface method's, whose receiver
// is the interface itself, and NewInterfaceType sets receivers of its own.
func substituteSignature(sig *types.Signature, env map[*types.TypeParam]types.Type) (*types.Signature, bool) {
	params, ok := substituteTuple(sig.Params(), env)
	if !ok {
		return nil, false
	}
	results, ok := substituteTuple(sig.Results(), env)
	if !ok {
		return nil, false
	}
	return types.NewSignatureType(nil, nil, nil, params, results, sig.Variadic()), true
}

func substituteTuple(tuple *types.Tuple, env map[*types.TypeParam]types.Type) (*types.Tuple, bool) {
	vars := make([]*types.Var, tuple.Len())
	for i := range tuple.Len() {
		field := tuple.At(i)
		typ, ok := substitute(field.Type(), env)
		if !ok {
			return nil, false
		}
		vars[i] = types.NewVar(field.Pos(), field.Pkg(), field.Name(), typ)
	}
	return types.NewTuple(vars...), true
}

func substituteStruct(strct *types.Struct, env map[*types.TypeParam]types.Type) (types.Type, bool) {
	fields := make([]*types.Var, strct.NumFields())
	tags := make([]string, strct.NumFields())
	for i := range strct.NumFields() {
		field := strct.Field(i)
		typ, ok := substitute(field.Type(), env)
		if !ok {
			return nil, false
		}
		fields[i] = types.NewField(field.Pos(), field.Pkg(), field.Name(), typ, field.Embedded())
		tags[i] = strct.Tag(i)
	}
	return types.NewStruct(fields, tags), true
}

func substituteInterface(iface *types.Interface, env map[*types.TypeParam]types.Type) (types.Type, bool) {
	methods := make([]*types.Func, 0, iface.NumExplicitMethods())
	for method := range iface.ExplicitMethods() {
		sig, ok := substituteSignature(method.Signature(), env)
		if !ok {
			return nil, false
		}
		methods = append(methods, types.NewFunc(method.Pos(), method.Pkg(), method.Name(), sig))
	}
	embeddeds := make([]types.Type, 0, iface.NumEmbeddeds())
	for embedded := range iface.EmbeddedTypes() {
		substituted, ok := substitute(embedded, env)
		if !ok {
			return nil, false
		}
		embeddeds = append(embeddeds, substituted)
	}
	substituted := types.NewInterfaceType(methods, embeddeds)
	substituted.Complete()
	return substituted, true
}

// parametricType reports whether t mentions a type parameter anywhere. A named
// type is judged by its type arguments alone: Go rejects a local type
// declaration that uses an enclosing type parameter, so arguments are the only
// way a named type can carry one, and not expanding underlying structure
// is also what keeps the walk finite on recursive types. Signatures are walked
// without their receiver for the same reason — an interface method's receiver
// is the interface itself.
func parametricType(t types.Type) bool {
	switch t := types.Unalias(t).(type) {
	case *types.TypeParam:
		return true
	case *types.Named:
		return parametricNamed(t)
	case *types.Pointer:
		return parametricType(t.Elem())
	case *types.Slice:
		return parametricType(t.Elem())
	case *types.Array:
		return parametricType(t.Elem())
	case *types.Chan:
		return parametricType(t.Elem())
	case *types.Map:
		return parametricType(t.Key()) || parametricType(t.Elem())
	case *types.Signature:
		return parametricTuple(t.Params()) || parametricTuple(t.Results())
	case *types.Tuple:
		return parametricTuple(t)
	case *types.Struct:
		return parametricStruct(t)
	case *types.Interface:
		return parametricInterface(t)
	case *types.Union:
		return parametricUnion(t)
	}
	return false
}

// parametricNamed treats a generic name carrying no type arguments
// as parametric, so a shape substitution has no arguments to rebuild lands
// in substitute's failure path rather than passing through unchanged.
func parametricNamed(named *types.Named) bool {
	if named.TypeParams().Len() > 0 && named.TypeArgs().Len() == 0 {
		return true
	}
	for arg := range named.TypeArgs().Types() {
		if parametricType(arg) {
			return true
		}
	}
	return false
}

func parametricTuple(tuple *types.Tuple) bool {
	for field := range tuple.Variables() {
		if parametricType(field.Type()) {
			return true
		}
	}
	return false
}

func parametricStruct(strct *types.Struct) bool {
	for field := range strct.Fields() {
		if parametricType(field.Type()) {
			return true
		}
	}
	return false
}

func parametricInterface(iface *types.Interface) bool {
	for method := range iface.ExplicitMethods() {
		if parametricType(method.Type()) {
			return true
		}
	}
	for embedded := range iface.EmbeddedTypes() {
		if parametricType(embedded) {
			return true
		}
	}
	return false
}

func parametricUnion(union *types.Union) bool {
	for term := range union.Terms() {
		if parametricType(term.Type()) {
			return true
		}
	}
	return false
}
