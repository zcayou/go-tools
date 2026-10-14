package engine

import (
	"go/types"
	"iter"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// errorInterface is the predeclared error type.
var errorInterface, _ = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

// errorNamed returns the error type t is a value of, through one pointer,
// or nil when t is no error type. An error type is a defined, non-interface
// type that has Error() string in the method set of T or *T. A generic error
// type is returned as its declaration, where it is judged.
func errorNamed(t types.Type) *types.Named {
	named, ok := types.Unalias(pointee(t)).(*types.Named)
	if !ok || types.IsInterface(named) {
		return nil
	}
	named = named.Origin()
	obj, _, _ := types.LookupFieldOrMethod(types.NewPointer(named), true, named.Obj().Pkg(), "Error")
	method, ok := obj.(*types.Func)
	if !ok {
		return nil
	}
	sig := method.Signature()
	if sig.Params().Len() != 0 || sig.Results().Len() != 1 ||
		!types.Identical(sig.Results().At(0).Type(), types.Typ[types.String]) {
		return nil
	}
	return named
}

// errorValued reports whether t is an interface type an error can be held in:
// one whose method set error's satisfies.
func errorValued(t types.Type) bool {
	return !typeParam(t) && types.IsInterface(t) && types.Implements(t, errorInterface)
}

// pointee is the type a pointer type points to, or t itself for any other.
func pointee(t types.Type) types.Type {
	if pointer, ok := types.Unalias(t).(*types.Pointer); ok {
		return pointer.Elem()
	}
	return t
}

// typeParam reports whether t is a type parameter.
func typeParam(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.TypeParam)
	return ok
}

// nameable reports whether a caller outside the module can identify an error
// type: an exported error type of an API package, one an exported alias
// of an API package names, any error type of a trusted package, or, while
// a base is set, one carrying the base.
func (a *analysis) nameable(named *types.Named) bool {
	obj := named.Origin().Obj()
	pkg := obj.Pkg()
	switch {
	case pkg == nil:
		return false
	case a.trusted(pkg), a.aliased[named.Origin()]:
		return true
	case a.module.Package(pkg) != nil && a.api[pkg.Path()] && obj.Exported() && obj.Parent() == pkg.Scope():
		return true
	default:
		return a.base != nil && a.carries(named)
	}
}

// exportedAliases returns the types the exported aliases of API packages
// name, through one pointer, as their origins. A caller names a type through
// an alias as well as through its declaration.
func (a *analysis) exportedAliases() map[*types.Named]bool {
	aliased := map[*types.Named]bool{}
	for _, pkg := range a.module.Packages {
		if !a.api[pkg.PkgPath] {
			continue
		}
		for _, name := range pkg.Types.Scope().Names() {
			obj, ok := pkg.Types.Scope().Lookup(name).(*types.TypeName)
			if !ok || !obj.IsAlias() || !obj.Exported() {
				continue
			}
			if named, ok := types.Unalias(pointee(obj.Type())).(*types.Named); ok {
				aliased[named.Origin()] = true
			}
		}
	}
	return aliased
}

// carries reports whether an error type carries the base: embeds a struct base
// by value, directly or through a chain of embeddings by value, or implements
// an interface base through T or *T.
func (a *analysis) carries(named *types.Named) bool {
	named = named.Origin()
	if iface, ok := a.base.Underlying().(*types.Interface); ok {
		t := ownInstance(named)
		return types.Implements(t, iface) || types.Implements(types.NewPointer(t), iface)
	}
	return a.embedsBase(named, map[*types.Named]bool{})
}

// ownInstance returns a generic type instantiated with its own type parameters,
// which has the methods the declaration has with their signatures as written,
// or any other type unchanged. Whether a type implements an interface
// is unspecified for an uninstantiated generic type.
func ownInstance(named *types.Named) types.Type {
	params := named.TypeParams()
	if params.Len() == 0 {
		return named
	}
	args := make([]types.Type, 0, params.Len())
	for param := range params.TypeParams() {
		args = append(args, param)
	}
	instance, err := types.Instantiate(nil, named, args, false)
	if err != nil {
		return named
	}
	return instance
}

func (a *analysis) embedsBase(named *types.Named, seen map[*types.Named]bool) bool {
	named = named.Origin()
	if seen[named] {
		return false
	}
	seen[named] = true
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for field := range st.Fields() {
		if !field.Embedded() {
			continue
		}
		embedded, ok := types.Unalias(field.Type()).(*types.Named)
		if !ok {
			continue
		}
		if embedded.Origin() == a.base.Origin() || a.embedsBase(embedded, seen) {
			return true
		}
	}
	return false
}

// closed reports whether a module interface is one whose implementations
// the module sees all of: unexported, declared in a package no other module can
// import, or carrying an unexported method.
func closed(named *types.Named) bool {
	obj := named.Obj()
	if !obj.Exported() || !wholemodule.Public(obj.Pkg().Path(), obj.Pkg().Name()) {
		return true
	}
	iface, _ := named.Underlying().(*types.Interface)
	return iface != nil && hasUnexportedMethod(iface)
}

// interfaceMethods yields the methods of an interface type, and nothing for any
// other type.
func interfaceMethods(t types.Type) iter.Seq[*types.Func] {
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return func(func(*types.Func) bool) {}
	}
	return iface.Methods()
}

func hasUnexportedMethod(iface *types.Interface) bool {
	for method := range iface.Methods() {
		if !method.Exported() {
			return true
		}
	}
	return false
}

// typeName renders a declared type as its package's name and its own.
func typeName(named *types.Named) string {
	obj := named.Origin().Obj()
	if obj.Pkg() == nil {
		return obj.Name()
	}
	return obj.Pkg().Name() + "." + obj.Name()
}

// receiverName is the name of the type a method is declared on, or ""
// for a function.
func receiverName(fn *types.Func) string {
	recv := fn.Signature().Recv()
	if recv == nil {
		return ""
	}
	if named, ok := types.Unalias(pointee(recv.Type())).(*types.Named); ok {
		return named.Origin().Obj().Name()
	}
	return ""
}

// funcName names a module function as a message about its own package does:
// Name, or Type.Name for a method.
func funcName(fn *types.Func) string {
	if recv := receiverName(fn); recv != "" {
		return recv + "." + fn.Name()
	}
	return fn.Name()
}

// foreignName names a foreign function, method or variable qualified by its
// package's name.
func foreignName(obj types.Object) string {
	qualifier := ""
	if obj.Pkg() != nil {
		qualifier = obj.Pkg().Name() + "."
	}
	if fn, ok := obj.(*types.Func); ok {
		if recv := receiverName(fn); recv != "" {
			return qualifier + recv + "." + fn.Name()
		}
	}
	return qualifier + obj.Name()
}

// errorResults reports whether a signature has a result an error can
// be returned in.
func errorResults(sig *types.Signature) bool {
	for result := range sig.Results().Variables() {
		if errorValued(result.Type()) {
			return true
		}
	}
	return false
}
