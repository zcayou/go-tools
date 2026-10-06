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
)

// declaration is one candidate finding, before the API surface and any nolint
// directive have had their say.
type declaration struct {
	pos      token.Position
	verdict  Verdict
	name     string
	kind     Kind
	pkg      string
	exported bool
	// owner is the declaration key of a method's receiver type, empty for every
	// other kind.
	owner string
}

// span is a half-open source region, position-keyed so it compares across
// test-variant packages that re-parse the same file.
type span struct {
	file       string
	start, end int
}

// spanIndex groups spans by file. A membership test then scans only the spans
// that could contain the position, rather than every span the run accumulated
// across every package.
type spanIndex map[string][]span

func (i spanIndex) add(spans ...span) {
	for _, sp := range spans {
		i[sp.file] = append(i[sp.file], sp)
	}
}

func (i spanIndex) contains(pos token.Position) bool {
	return slices.ContainsFunc(i[pos.Filename], func(sp span) bool {
		return pos.Offset >= sp.start && pos.Offset < sp.end
	})
}

// position resolves a token.Pos without applying //line directives. A caller
// places a finding by looking up the file it parsed, and a directive renames
// a region to a name no caller holds; one file can carry several such names, so
// the unrewritten name is the only one that stays a valid key. The byte offset
// a caller remaps through is the same either way.
func position(fset *token.FileSet, pos token.Pos) token.Position {
	return fset.PositionFor(pos, false)
}

// declKey identifies a declaration by where it is written. A byte offset rather
// than a line keeps two declarations on one line apart, and the key compares
// equal across the several packages that re-parse the same file.
func declKey(pos token.Position) string {
	return pos.Filename + ":" + strconv.Itoa(pos.Offset)
}

func toSpan(fset *token.FileSet, start, end token.Pos) span {
	startPos := position(fset, start)
	return span{file: startPos.Filename, start: startPos.Offset, end: position(fset, end).Offset}
}

func within(pos token.Position, spans []span) bool {
	return slices.ContainsFunc(spans, func(sp span) bool {
		return sp.file == pos.Filename && pos.Offset >= sp.start && pos.Offset < sp.end
	})
}

func usedOutsideSpans(uses []token.Position, selfSpans []span, blankSpans spanIndex) bool {
	for _, use := range uses {
		if within(use, selfSpans) || blankSpans.contains(use) {
			continue
		}
		return true
	}
	return false
}

// blankAssertionSpans returns the source spans of an inert blank spec,
// with function literal interiors removed. Only an inert `var _ T = ...`
// neutralizes the references it contains: it exists to make the compiler check
// something, so naming a declaration there is not a use of it. A spec whose
// value is computed runs that computation — Ginkgo's `var _ = Describe("...",
// func() { ... })` uses everything it names — and a function literal an inert
// spec carries is live source all the same, because that body runs when
// something calls it.
func blankAssertionSpans(pkg *packages.Package, spec *ast.ValueSpec) []span {
	if !inertAssertion(pkg, spec) {
		return nil
	}
	fset := pkg.Fset
	full := toSpan(fset, spec.Pos(), spec.End())

	var lits []span
	ast.Inspect(spec, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			lits = append(lits, toSpan(fset, lit.Pos(), lit.End()))
			return false
		}
		return true
	})
	if len(lits) == 0 {
		return []span{full}
	}

	slices.SortFunc(lits, func(a, b span) int { return cmp.Compare(a.start, b.start) })

	var spans []span
	cursor := full.start
	for _, lit := range lits {
		if lit.start > cursor {
			spans = append(spans, span{file: full.file, start: cursor, end: lit.start})
		}
		cursor = lit.end
	}
	if cursor < full.end {
		spans = append(spans, span{file: full.file, start: cursor, end: full.end})
	}
	return spans
}

// inertAssertion reports whether the spec is a compile-time assertion rather
// than an initializer that runs. It needs both halves: the declared type
// is what the assertion checks against, and an inert value is one the compiler
// can satisfy without calling anything.
func inertAssertion(pkg *packages.Package, spec *ast.ValueSpec) bool {
	if spec.Type == nil {
		return false
	}
	for _, value := range spec.Values {
		if !inertValue(pkg, value) {
			return false
		}
	}
	return true
}

func inertValue(pkg *packages.Package, expr ast.Expr) bool {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.CompositeLit, *ast.BasicLit, *ast.Ident, *ast.SelectorExpr:
		return true
	case *ast.UnaryExpr:
		return expr.Op == token.AND && inertValue(pkg, expr.X)
	case *ast.StarExpr:
		return inertValue(pkg, expr.X)
	case *ast.CallExpr:
		// A conversion is inert; a call is not.
		return pkg.TypesInfo.Types[expr.Fun].IsType()
	default:
		return false
	}
}

func sortDeclarations(decls []declaration) {
	slices.SortFunc(decls, func(a, b declaration) int {
		if c := strings.Compare(a.pos.Filename, b.pos.Filename); c != 0 {
			return c
		}
		return cmp.Compare(a.pos.Offset, b.pos.Offset)
	})
}

// fileFacts are the per-package source properties several scans consult. Each
// is derived by walking every file of the package, so they are computed once
// and threaded rather than recomputed per scan.
type fileFacts struct {
	// generated holds the files nothing is reported in, keyed the way declarations
	// carry them.
	generated map[string]bool
	// linknamed holds the package-level function names a //go:linkname directive
	// publishes under another package's symbol.
	linknamed map[string]bool
}

func newFileFacts(pkgs []*packages.Package) map[*packages.Package]fileFacts {
	facts := make(map[*packages.Package]fileFacts, len(pkgs))
	for _, pkg := range pkgs {
		facts[pkg] = fileFacts{generated: generatedFiles(pkg), linknamed: linknamedFuncs(pkg)}
	}
	return facts
}

// generatedFiles reports which of the package's parsed files nothing should
// be reported in. A cgo package is the exception the GoFiles check exists for:
// the loader hands back a rewrite of every hand-written file, carrying
// the rewriter's generated header over //line directives that point back
// at the package's own source. Taking that header at face value would silently
// report nothing at all in a cgo package.
func generatedFiles(pkg *packages.Package) map[string]bool {
	sources := make(map[string]bool, len(pkg.GoFiles))
	for _, name := range pkg.GoFiles {
		sources[name] = true
	}
	generated := map[string]bool{}
	for _, file := range pkg.Syntax {
		if !ast.IsGenerated(file) {
			continue
		}
		name := position(pkg.Fset, file.Pos()).Filename
		if !sources[name] && sources[pkg.Fset.Position(file.Pos()).Filename] {
			continue
		}
		generated[name] = true
	}
	return generated
}

// linknamedFuncs returns the package-level function names a //go:linkname
// directive publishes under another package's symbol. The body then runs under
// a name no reference in source mentions, so the function is an entry point
// rather than dead code. Every comment in the file is read, not only
// the declaration's doc comment, because a blank line between the two detaches
// the directive from the declaration without disabling it.
func linknamedFuncs(pkg *packages.Package) map[string]bool {
	linknamed := map[string]bool{}
	for _, file := range pkg.Syntax {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				// Two operands is the push form, which provides the local symbol under
				// the target name. One operand is the pull form, whose declaration has no
				// body and so is never a candidate.
				if fields := strings.Fields(comment.Text); len(fields) == 3 && fields[0] == "//go:linkname" {
					linknamed[fields[1]] = true
				}
			}
		}
	}
	return linknamed
}

// receiverBase unwraps a receiver expression down to the type name, past
// the pointer, parentheses, and type parameters a generic receiver carries.
func receiverBase(expr ast.Expr) *ast.Ident {
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.ParenExpr:
			expr = e.X
		case *ast.Ident:
			return e
		default:
			return nil
		}
	}
}

// receiverTypeKey returns the declaration key of a method's receiver type, so
// a finding can be tied to the type it hangs off. Package-level functions have
// none.
func receiverTypeKey(pkg *packages.Package, decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return ""
	}
	ident := receiverBase(decl.Recv.List[0].Type)
	if ident == nil {
		return ""
	}
	obj, ok := pkg.TypesInfo.Uses[ident]
	if !ok {
		return ""
	}
	return declKey(position(pkg.Fset, obj.Pos()))
}

func allBlank(names []*ast.Ident) bool {
	return !slices.ContainsFunc(names, func(name *ast.Ident) bool {
		return name.Name != "_"
	})
}

func lookupMethodByName(methods *types.MethodSet, name string) *types.Selection {
	for selection := range methods.Methods() {
		if selection.Obj().Name() == name {
			return selection
		}
	}
	return nil
}

// resolveMethod finds the implementation in set that satisfies method.
// The by-name fallback covers an interface method whose package differs from
// the implementation's, where a qualified lookup misses; the signature check
// keeps that fallback from matching an unrelated method of the same name.
func resolveMethod(set *types.MethodSet, method *types.Func) *types.Func {
	return resolveMethodAs(set, method, method.Signature())
}

// resolveMethodAs is resolveMethod with the signature the implementation must
// carry given separately, for callers that substituted the method's written
// signature first.
func resolveMethodAs(set *types.MethodSet, method *types.Func, expected *types.Signature) *types.Func {
	selection := set.Lookup(method.Pkg(), method.Name())
	if selection == nil {
		selection = lookupMethodByName(set, method.Name())
	}
	if selection == nil {
		return nil
	}
	implementation, ok := selection.Obj().(*types.Func)
	if !ok || !types.IdenticalIgnoreTags(implementation.Type(), expected) {
		return nil
	}
	return implementation
}

// implementsInterface reports whether the method's receiver satisfies iface,
// trying the pointer form as well so a value receiver in a pointer-satisfied
// method set still counts.
func implementsInterface(method *types.Func, iface *types.Interface) bool {
	recv := method.Signature().Recv()
	if recv == nil {
		return false
	}
	if types.Implements(recv.Type(), iface) {
		return true
	}
	if _, ok := recv.Type().(*types.Pointer); !ok {
		return types.Implements(types.NewPointer(recv.Type()), iface)
	}
	return false
}

// implementsAt reports whether method's receiver implements iface, weighing
// a generic receiver as written and at each instantiation inst holds of it.
// As written it satisfies an interface only while its methods' signatures leave
// its type parameters out, and then every instantiation does; a method
// mentioning one — compile() (Schema[T], error) — matches an instantiated
// or a concrete interface only once the receiver is instantiated too,
// and the instantiations the program builds are the ones it is known to hold.
// A receiver the program never instantiates satisfies only where the written
// form already does.
func implementsAt(method *types.Func, iface *types.Interface, inst *instantiations) bool {
	if implementsInterface(method, iface) {
		return true
	}
	recv := method.Signature().Recv()
	if recv == nil {
		return false
	}
	named, ok := heldForm(recv.Type()).(*types.Named)
	if !ok || named.TypeParams().Len() == 0 {
		return false
	}
	for _, vector := range inst.vectors(named.Obj()) {
		instance, err := types.Instantiate(nil, named, vector, false)
		if err != nil {
			continue
		}
		// The pointer's method set holds the value receiver's methods as well.
		if types.Implements(types.NewPointer(instance), iface) {
			return true
		}
	}
	return false
}
