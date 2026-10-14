package engine

import (
	"go/types"
	"maps"
)

// judgedFunctions returns the functions unstructured-return judges, each
// with a result an error can be returned in:
//
//   - the exported functions of API packages;
//   - the exported methods in the method set of an exported type of an API
//     package, a promoted one at its declaration;
//   - the methods implementing the exported methods of an exported interface
//     of an API package, which consumers call through the interface;
//   - the methods a module type answers a trusted interface with, once module
//     code converts the type to it.
//
// It takes out Unwrap on an error type, the implementations of a protocols
// entry, and the methods a module type answers a foreign, untrusted interface
// with, which that interface's own contract governs. A protocols entry taking
// nothing out is a [*ConfigError].
func (a *analysis) judgedFunctions() (map[*types.Func]bool, error) {
	judged := map[*types.Func]bool{}
	add := func(fn *types.Func) {
		if fn == nil {
			return
		}
		fn = fn.Origin()
		if !a.module.Declares(fn) || !errorResults(fn.Signature()) {
			return
		}
		if body := a.idx.declared[fn]; body == nil || len(body.Blocks) == 0 {
			return
		}
		judged[fn] = true
	}

	for _, pkg := range a.module.Packages {
		if a.api[pkg.PkgPath] {
			a.judgeAPI(pkg.Types, add)
		}
	}
	a.idx.eachConversion(func(iface types.Type, concretes []types.Type) {
		named, ok := types.Unalias(iface).(*types.Named)
		if !ok || !a.trusted(named.Obj().Pkg()) {
			return
		}
		for _, concrete := range concretes {
			for method := range interfaceMethods(iface) {
				add(methodOf(concrete, method))
			}
		}
	})

	exempt, err := a.exemptions(judged)
	if err != nil {
		return nil, err
	}
	for fn := range exempt {
		delete(judged, fn)
	}
	return judged, nil
}

// judgeAPI adds an API package's exported functions, the exported methods
// of its exported types, and the methods implementing its exported interfaces.
// An exported alias exposes the type it names as the type's own declaration
// would.
func (a *analysis) judgeAPI(pkg *types.Package, add func(*types.Func)) {
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		switch obj := scope.Lookup(name).(type) {
		case *types.Func:
			if obj.Exported() {
				add(obj)
			}
		case *types.TypeName:
			if !obj.Exported() {
				continue
			}
			named, ok := types.Unalias(obj.Type()).(*types.Named)
			if !ok {
				continue
			}
			if types.IsInterface(named) {
				a.judgeImplementations(named, add)
				continue
			}
			for sel := range types.NewMethodSet(types.NewPointer(named)).Methods() {
				if fn, ok := sel.Obj().(*types.Func); ok && fn.Exported() {
					add(fn)
				}
			}
		}
	}
}

// judgeImplementations adds the methods each module type implementing
// an exported API interface implements its exported methods with.
func (a *analysis) judgeImplementations(iface *types.Named, add func(*types.Func)) {
	for _, impl := range a.implementing(iface) {
		for method := range interfaceMethods(impl.iface) {
			if method.Exported() {
				add(methodOf(impl.concrete, method))
			}
		}
	}
}

// implementation is a concrete type implementing an interface type: the named
// interface itself, or for a generic one, an instance of it.
type implementation struct {
	concrete types.Type
	iface    types.Type
}

// implementing returns the module types implementing iface, as the type whose
// method set does, T or *T, along with the concrete types module code converts
// to it, which covers the instances of generic types. A generic interface
// is implemented through its instances, so for one it returns the concrete
// types converted to each instance module code converts to.
func (a *analysis) implementing(iface *types.Named) []implementation {
	var found []implementation
	if iface.TypeParams().Len() > 0 {
		a.idx.eachConversion(func(converted types.Type, concretes []types.Type) {
			named, ok := types.Unalias(converted).(*types.Named)
			if !ok || named.Origin() != iface.Origin() || named == iface.Origin() {
				return
			}
			for _, concrete := range concretes {
				found = append(found, implementation{concrete: concrete, iface: named})
			}
		})
		return found
	}
	underlying, ok := iface.Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	for _, named := range a.idx.types {
		if named.TypeParams().Len() > 0 {
			continue
		}
		switch {
		case types.Implements(named, underlying):
			found = append(found, implementation{concrete: named, iface: iface})
		case types.Implements(types.NewPointer(named), underlying):
			found = append(found, implementation{concrete: types.NewPointer(named), iface: iface})
		}
	}
	for _, concrete := range a.idx.implementations(iface) {
		found = append(found, implementation{concrete: concrete, iface: iface})
	}
	return found
}

// exemptions returns the judged functions no rule judges after all. A protocol
// counts as exempting a method only when nothing else already does.
func (a *analysis) exemptions(judged map[*types.Func]bool) (map[*types.Func]bool, error) {
	exempt := map[*types.Func]bool{}
	for fn := range judged {
		if unwrap(fn) {
			exempt[fn] = true
		}
	}
	a.idx.eachConversion(func(iface types.Type, concretes []types.Type) {
		named, ok := types.Unalias(iface).(*types.Named)
		if !ok || named.Obj().Pkg() == nil || a.module.Package(named.Obj().Pkg()) != nil || a.trusted(named.Obj().Pkg()) {
			return
		}
		for _, concrete := range concretes {
			for method := range interfaceMethods(iface) {
				if fn := methodOf(concrete, method); fn != nil {
					exempt[fn.Origin()] = true
				}
			}
		}
	})
	protocolled := map[*types.Func]bool{}
	for _, p := range a.protocols {
		exempts := false
		for _, impl := range a.implementing(p.iface) {
			fn := methodOf(impl.concrete, p.method)
			if fn == nil {
				continue
			}
			exempts = exempts || judged[fn.Origin()] && !exempt[fn.Origin()]
			protocolled[fn.Origin()] = true
		}
		if !exempts {
			return nil, &ConfigError{Setting: SettingProtocols, Problem: ProblemExemptsNothing, Value: p.entry}
		}
	}
	maps.Copy(exempt, protocolled)
	return exempt, nil
}

// unwrap reports whether fn is Unwrap() error or Unwrap() []error on an error
// type: it hands a caller the cause to identify, and is not itself one.
func unwrap(fn *types.Func) bool {
	sig := fn.Signature()
	if fn.Name() != "Unwrap" || sig.Recv() == nil || sig.Params().Len() != 0 || sig.Results().Len() != 1 ||
		errorNamed(sig.Recv().Type()) == nil {
		return false
	}
	result := sig.Results().At(0).Type()
	if slice, ok := result.(*types.Slice); ok {
		result = slice.Elem()
	}
	return types.Identical(result, types.Universe.Lookup("error").Type())
}

// methodOf returns the concrete method t's method set answers an interface
// method with, or nil.
func methodOf(t types.Type, method *types.Func) *types.Func {
	obj, _, _ := types.LookupFieldOrMethod(t, false, method.Pkg(), method.Name())
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}
	if recv := fn.Signature().Recv(); recv != nil && types.IsInterface(recv.Type()) {
		return nil
	}
	return fn
}
