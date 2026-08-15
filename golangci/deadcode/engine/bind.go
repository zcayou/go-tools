package engine

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// interfaceBindScan records the exported methods a concrete type contributes
// to an interface the program actually binds it to: a conversion
// to that interface, a type assertion that discovers it, or a generic
// instantiation that constrains on it. Participation is all a static analysis
// can observe once the caller is a library — it never sees log/slog invoke
// the handler it was given — so a bind counts as a use of every method
// the interface declares.
//
// Conversions are read from the SSA program, where generics are already
// instantiated, so a concrete type reaching a generic interface is seen
// as the instantiated interface rather than the parameterized one no concrete
// type implements. Assertions are read from the syntax of every loaded package,
// dependencies included, because a library's discovery of an optional
// capability happens in its own source and SSA bodies are built only
// for the analyzed packages.
type interfaceBindScan struct {
	credited map[string]bool
}

func newInterfaceBindScan(
	prog *ssa.Program,
	loaded []*packages.Package,
	refs *methodReferenceScan,
	analyzed map[string]bool,
	ev *evidence,
	inst *instantiations,
	flows *interfaceFlows,
	v view,
) *interfaceBindScan {
	scan := &interfaceBindScan{credited: map[string]bool{}}

	for operand, iface := range ev.conversions {
		scan.creditConversion(prog, refs, flows, analyzed, operand, iface)
	}
	for _, asserted := range assertedInterfaces(loaded, v) {
		scan.creditAssertion(prog.Fset, refs, flows, analyzed, ev, inst, asserted)
	}
	scan.creditInstantiations(prog.Fset, refs, flows, analyzed, inst)

	return scan
}

func (s *interfaceBindScan) bound(key string) bool {
	return s.credited[key]
}

// creditConversion credits the methods concrete contributes to iface.
// The method set resolves each interface method to the implementation
// that would run, so an override is credited over the embedded default
// it shadows, and an embedded default is credited for the types that do not
// override it.
func (s *interfaceBindScan) creditConversion(
	prog *ssa.Program,
	refs *methodReferenceScan,
	flows *interfaceFlows,
	analyzed map[string]bool,
	concrete, iface types.Type,
) {
	methods, ok := iface.Underlying().(*types.Interface)
	if !ok || methods.NumMethods() == 0 {
		return
	}
	set := prog.MethodSets.MethodSet(concrete)
	for method := range methods.Methods() {
		if !confersUse(prog.Fset, refs, flows, analyzed, iface, method) {
			continue
		}
		if implementation := resolveMethod(set, method); implementation != nil {
			s.credited[declKey(position(prog.Fset, implementation.Pos()))] = true
		}
	}
}

// creditAssertion credits every declared method the asserted interface could
// dispatch to. An assertion names no concrete type, so the candidates
// are the declared methods sharing the interface method's name whose receiver
// both satisfies the whole interface and is a type the program materializes
// behind an interface somewhere. Without that second half the assertions
// a dependency makes about its own interfaces would credit anything shaped like
// them, and fmt alone asserts enough to spare every String, Error, and Format
// method ever written. The candidate pool is the reference scan's, which
// is unfiltered on purpose: an unexported method can only satisfy
// a same-package interface, which is exactly the within-package capability
// pattern, and an entry in a test or generated file is harmless because no
// verdict names it.
func (s *interfaceBindScan) creditAssertion(
	fset *token.FileSet,
	refs *methodReferenceScan,
	flows *interfaceFlows,
	analyzed map[string]bool,
	ev *evidence,
	inst *instantiations,
	asserted assertedType,
) {
	if asserted.enclosing == nil {
		s.creditAssertedInterface(fset, refs, flows, analyzed, ev, asserted.typ)
		return
	}
	// An asserted type written against the enclosing generic's type parameters
	// is specialized once per concrete vector the program instantiates
	// that generic with; an unresolvable vector or a failed substitution credits
	// nothing. Substituting the whole type is safe here: an assertion target
	// is necessarily a basic interface, so it never carries union terms.
	for _, vector := range inst.vectors(asserted.enclosing.obj) {
		if len(vector) != len(asserted.enclosing.params) {
			continue
		}
		specialized, ok := substitute(asserted.typ, environment(asserted.enclosing.params, vector))
		if !ok {
			continue
		}
		s.creditAssertedInterface(fset, refs, flows, analyzed, ev, specialized)
	}
}

func (s *interfaceBindScan) creditAssertedInterface(
	fset *token.FileSet,
	refs *methodReferenceScan,
	flows *interfaceFlows,
	analyzed map[string]bool,
	ev *evidence,
	iface types.Type,
) {
	methods, ok := iface.Underlying().(*types.Interface)
	if !ok || methods.NumMethods() == 0 {
		return
	}
	for method := range methods.Methods() {
		if !confersUse(fset, refs, flows, analyzed, iface, method) {
			continue
		}
		for _, implementation := range refs.concreteMethodsByName[method.Name()] {
			if ev.materialized(implementation.fn) && implementsInterface(implementation.fn, methods) {
				s.credited[implementation.key] = true
			}
		}
	}
}

// creditInstantiations credits the methods a type argument contributes
// to an interface constraint. A constraint is satisfied, never converted, so no
// conversion records it — but the compiler verified the type argument against
// it at every instantiation, which is the same evidence a conversion carries.
// Satisfaction being given, the constraint's method names are enough to find
// the implementations the instantiated body calls. The instantiations
// are the resolved ones: a site written inside another generic credits under
// each concrete vector the fixpoint carried down to it, rather than
// as the parametric form no concrete method could match.
func (s *interfaceBindScan) creditInstantiations(
	fset *token.FileSet,
	refs *methodReferenceScan,
	flows *interfaceFlows,
	analyzed map[string]bool,
	inst *instantiations,
) {
	for obj, vectors := range inst.all {
		params := typeParams(obj)
		if params == nil {
			continue
		}
		for _, vector := range vectors {
			if len(vector) != params.Len() {
				continue
			}
			env := environment(slices.Collect(params.TypeParams()), vector)
			for i, argument := range vector {
				s.creditConstraint(fset, refs, flows, analyzed, argument, params.At(i).Constraint(), env)
			}
		}
	}
}

// creditConstraint matches each constraint method against the concrete
// argument's method set, substituting the method's signature under
// the instantiation's environment first — the constraint interface type itself
// is never substituted, so constraint-only shapes such as union terms never
// reach substitution. A method whose signature mentions no type parameter
// is compared exactly as written.
func (s *interfaceBindScan) creditConstraint(
	fset *token.FileSet,
	refs *methodReferenceScan,
	flows *interfaceFlows,
	analyzed map[string]bool,
	argument, constraint types.Type,
	env map[*types.TypeParam]types.Type,
) {
	methods, ok := constraint.Underlying().(*types.Interface)
	if !ok || methods.NumMethods() == 0 {
		return
	}
	sets := []*types.MethodSet{types.NewMethodSet(argument)}
	if _, pointer := argument.(*types.Pointer); !pointer {
		sets = append(sets, types.NewMethodSet(types.NewPointer(argument)))
	}
	for method := range methods.Methods() {
		if !confersUse(fset, refs, flows, analyzed, constraint, method) {
			continue
		}
		expected, ok := substituteSignature(method.Signature(), env)
		if !ok {
			continue
		}
		for _, set := range sets {
			if implementation := resolveMethodAs(set, method, expected); implementation != nil {
				s.credited[declKey(position(fset, implementation.Pos()))] = true
				break
			}
		}
	}
}

// assertedType is one interface type the loaded packages assert to,
// with the generic context needed to make it concrete: enclosing
// is the innermost generic declaration around the assertion when the written
// type references its type parameters, nil otherwise. Deduplication keys
// the written form together with that context, so the same spelling under two
// different generics stays two records — each substitutes under its own
// parameters.
type assertedType struct {
	typ       types.Type
	enclosing *enclosingGeneric
}

// assertedInterfaces returns every interface type the loaded packages assert
// to in files the view admits, dependencies included. A library discovers
// an optional capability by asserting — errors.Is looks for an Unwrap,
// a decoder looks for an Unmarshaler — and that assertion is the only evidence
// in source that the methods behind it run. Dependencies load without their
// tests, so masking only ever bites the analyzed packages' own test files.
func assertedInterfaces(loaded []*packages.Package, v view) []assertedType {
	var asserted []assertedType
	seen := map[string]bool{}
	packages.Visit(loaded, nil, func(pkg *packages.Package) {
		if pkg.TypesInfo == nil || !v.admitsPackage(pkg.PkgPath) {
			return
		}
		for _, file := range pkg.Syntax {
			if !v.admitsFile(position(pkg.Fset, file.Pos()).Filename) {
				continue
			}
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.FuncDecl:
					asserted = collectAsserted(pkg, decl, funcEnclosing(pkg, decl), seen, asserted)
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						var enclosing *enclosingGeneric
						if typeSpec, ok := spec.(*ast.TypeSpec); ok {
							enclosing = typeSpecEnclosing(pkg, typeSpec)
						}
						asserted = collectAsserted(pkg, spec, enclosing, seen, asserted)
					}
				}
			}
		}
	})
	return asserted
}

// collectAsserted gathers the assertion targets under one top-level
// declaration, attributing the enclosing generic only where the written type
// actually references type parameters — a concrete assertion inside a generic
// body needs no specialization and deduplicates globally.
func collectAsserted(
	pkg *packages.Package,
	node ast.Node,
	enclosing *enclosingGeneric,
	seen map[string]bool,
	asserted []assertedType,
) []assertedType {
	info := pkg.TypesInfo
	add := func(typ types.Type) {
		if typ == nil || !types.IsInterface(typ) {
			return
		}
		record := assertedType{typ: typ}
		key := typ.String()
		if enclosing != nil && parametricType(typ) {
			record.enclosing = enclosing
			key += "|" + declKey(position(pkg.Fset, enclosing.obj.Pos()))
		}
		if seen[key] {
			return
		}
		seen[key] = true
		asserted = append(asserted, record)
	}
	addExpr := func(expr ast.Expr) {
		// The guard of a type switch has no asserted type of its own.
		if expr != nil {
			add(info.Types[expr].Type)
		}
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.TypeAssertExpr:
			addExpr(n.Type)
		case *ast.TypeSwitchStmt:
			for _, statement := range n.Body.List {
				for _, expr := range statement.(*ast.CaseClause).List {
					addExpr(expr)
				}
			}
		case *ast.Ident:
			// reflect.TypeAssert is an assertion written as a call, and encoding/json,
			// encoding/xml and encoding/asn1 discover Marshaler and TextMarshaler
			// through nothing else. Only this one instantiation qualifies: an interface
			// reaching any generic function as a type argument is not an assertion,
			// it is a constraint satisfaction that creditInstantiations already weighs.
			instance, ok := info.Instances[n]
			if !ok || instance.TypeArgs.Len() == 0 {
				return true
			}
			if fn, isFunc := info.Uses[n].(*types.Func); isFunc && isReflectTypeAssert(fn) {
				add(instance.TypeArgs.At(0))
			}
		}
		return true
	})
	return asserted
}

func isReflectTypeAssert(fn *types.Func) bool {
	return fn.Name() == "TypeAssert" && fn.Pkg() != nil && fn.Pkg().Path() == "reflect"
}

// confersUse reports whether satisfying method through iface counts as a use
// of the implementation. An interface a dependency declares confers
// unconditionally: its call sites are beyond what this analysis reports on, so
// participation is the only evidence available. An interface the analyzed
// packages declare confers only while that interface method is itself used —
// selected directly, or reached through interface-to-interface flows — which
// keeps a dead interface method from laundering its implementations: the method
// is reported alongside them instead. An anonymous interface has no declaration
// to weigh and confers unconditionally.
func confersUse(
	fset *token.FileSet,
	refs *methodReferenceScan,
	flows *interfaceFlows,
	analyzed map[string]bool,
	iface types.Type,
	method *types.Func,
) bool {
	// An alias is a named type wearing another name, and since Go 1.23 it arrives
	// as its own node. Reading past it is what keeps an alias from turning
	// the rule off.
	named, ok := types.Unalias(iface).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || !analyzed[named.Obj().Pkg().Path()] {
		return true
	}
	key := declKey(position(fset, method.Pos()))
	if _, declared := refs.interfaceMethods[key]; !declared {
		return true
	}
	return flows.used(key)
}

func typeParams(obj types.Object) *types.TypeParamList {
	switch obj := obj.(type) {
	case *types.Func:
		return obj.Signature().TypeParams()
	case *types.TypeName:
		if named, ok := obj.Type().(*types.Named); ok {
			return named.TypeParams()
		}
	}
	return nil
}
