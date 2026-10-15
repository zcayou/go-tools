package spec

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
)

// assertionMethods apply a matcher to an assertion's actual value, each mapped
// to whether it negates the matcher.
var assertionMethods = map[string]bool{
	"To": false, "Should": false,
	"NotTo": true, "ToNot": true, "ShouldNot": true,
}

// assertionStarts take an assertion's actual value, as Gomega functions
// and as methods of a Gomega value, each mapped to the index of the argument
// holding it.
var assertionStarts = map[string]int{
	"Expect": 0, "Ω": 0, "ExpectWithOffset": 1,
	"Eventually": 0, "EventuallyWithOffset": 1, "Consistently": 0, "ConsistentlyWithOffset": 1,
}

// asyncStarts are the assertion starts that may take a context ahead
// of the actual value.
var asyncStarts = []string{"Eventually", "EventuallyWithOffset", "Consistently", "ConsistentlyWithOffset"}

// compositions are the matchers composing others, each of which keeps
// the polarity it is applied under.
var compositions = []string{"And", "Or", "SatisfyAll", "SatisfyAny"}

// stringMatchers match a string. Given to MatchError, each matches an error's
// text.
var stringMatchers = []string{"ContainSubstring", "HavePrefix", "HaveSuffix", "MatchRegexp", "Equal", "BeEquivalentTo"}

// sprints render their operands as fmt's %v does, which for an error is its
// text.
var sprints = []string{"Sprint", "Sprintf", "Sprintln"}

// gomegaMatcher judges a call to one of the Gomega matchers the rules name,
// under the polarity it is applied in.
func (f *fileChecker) gomegaMatcher(call *ast.CallExpr, stack []ast.Node) {
	name, ok := f.refers(call.Fun, gomegaPath)
	if !ok {
		return
	}
	switch name {
	case "HaveOccurred":
		if site := f.matcherContext(call, stack); !site.negated {
			f.unidentified(call.Pos(), "HaveOccurred says only that some error occurred; "+
				"assert which one, by its type and fields")
		}
	case "Succeed":
		if site := f.matcherContext(call, stack); site.negated {
			f.unidentified(call.Pos(), "a negated Succeed says only that some error occurred; "+
				"assert which one, by its type and fields")
		}
	case "BeNil":
		site := f.matcherContext(call, stack)
		if site.negated && !site.transformed && site.actual != nil && f.isError(site.actual) {
			f.unidentified(call.Pos(), "a negated BeNil of an error says only that some error occurred; "+
				"assert which one, by its type and fields")
		}
	case "MatchError":
		if len(call.Args) == 0 {
			return
		}
		site := f.matcherContext(call, stack)
		switch {
		case f.builtError(call.Args[0]):
			if !site.negated {
				f.textAsserted(call.Pos(), builtMessage, site.actual)
			}
		case f.positiveText(call.Args[0], site.negated):
			f.textAsserted(call.Pos(), textMessage, site.actual)
		}
	case "HaveField":
		if len(call.Args) == 2 {
			f.haveField(call, stack)
		}
	}
}

// haveField judges a HaveField whose path reads an error's text, or an enum
// field's rendering.
func (f *fileChecker) haveField(call *ast.CallExpr, stack []ast.Node) {
	lit, ok := ast.Unparen(call.Args[0]).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return
	}
	path, err := strconv.Unquote(lit.Value)
	if err != nil {
		return
	}
	site := f.matcherContext(call, stack)
	if !f.positiveLeaf(call.Args[1], site.negated) {
		return
	}
	if path == "Error()" {
		f.textAsserted(call.Pos(), textMessage, site.actual)
		return
	}
	if site.actual == nil || site.transformed {
		return
	}
	if e, ok := f.fieldRendering(f.pass.TypesInfo.TypeOf(site.actual), path); ok {
		f.tokenAsserted(call.Pos(), []enumRef{e})
	}
}

// gomegaAssertion judges an assertion whose actual value is an error's text.
func (f *fileChecker) gomegaAssertion(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return
	}
	negates, ok := assertionMethods[sel.Sel.Name]
	if !ok {
		return
	}
	actual, ok := f.actualOf(sel.X)
	if !ok {
		return
	}
	if !f.positiveLeaf(call.Args[0], negates) {
		return
	}
	if subjects, ok := f.textOf(actual); ok {
		f.textAsserted(actual.Pos(), textMessage, subjects...)
	}
	if enums := f.renderingOf(actual); len(enums) > 0 {
		f.tokenAsserted(actual.Pos(), enums)
	}
}

// matcherSite is where a matcher is applied.
type matcherSite struct {
	// negated reports whether the matcher is negated there.
	negated bool

	// actual is the actual value of the Gomega assertion applying it, or nil
	// when it is held outside any.
	actual ast.Expr

	// transformed reports that a WithTransform stands between the matcher
	// and the actual value, so the matcher sees what the transform made of it.
	transformed bool
}

// matcherContext walks out from a matcher to the assertion applying it: each
// enclosing Not flips its polarity, composed matchers keep it,
// and the assertion method's own negation applies last. A matcher held outside
// any assertion, as a table entry's is, has only its Nots to go by; the walk
// ends at the statement or function holding it.
func (f *fileChecker) matcherContext(call *ast.CallExpr, stack []ast.Node) matcherSite {
	var site matcherSite
	child := ast.Node(call)
	for _, node := range slices.Backward(stack) {
		switch node := node.(type) {
		case ast.Stmt, *ast.FuncLit, *ast.FuncDecl:
			return site
		case *ast.CallExpr:
			if name, ok := f.refers(node.Fun, gomegaPath); ok {
				switch name {
				case "Not":
					site.negated = !site.negated
				case "WithTransform":
					site.transformed = true
				}
				break
			}
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				break
			}
			if negates, ok := assertionMethods[sel.Sel.Name]; ok && len(node.Args) > 0 && node.Args[0] == child {
				site.negated = site.negated != negates
				site.actual, _ = f.actualOf(sel.X)
				return site
			}
		}
		child = node
	}
	return site
}

// actualOf returns the actual value of the assertion recv applies a matcher to:
// the argument of the Gomega call starting it, past whatever methods
// are chained after that call. A chained method never supplies the actual
// value: Expect(f()).Error() selects f's error result, which is not text.
func (f *fileChecker) actualOf(recv ast.Expr) (ast.Expr, bool) {
	for {
		call, ok := ast.Unparen(recv).(*ast.CallExpr)
		if !ok {
			return nil, false
		}
		if name, index, started := f.assertionStart(call); started {
			// Eventually and Consistently take a context ahead of the actual value.
			if slices.Contains(asyncStarts, name) && index+1 < len(call.Args) && f.isContext(call.Args[index]) {
				index++
			}
			if index >= len(call.Args) {
				return nil, false
			}
			return call.Args[index], true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil, false
		}
		recv = sel.X
	}
}

// assertionStart reports whether a call starts a Gomega assertion, which start
// it is, and at which argument its actual value sits. A start is a Gomega
// function, or a method of a Gomega value however it was made.
func (f *fileChecker) assertionStart(call *ast.CallExpr) (string, int, bool) {
	fn := f.funcOf(call.Fun)
	if !gomega(fn) {
		return "", 0, false
	}
	index, ok := assertionStarts[fn.Name()]
	return fn.Name(), index, ok
}

// builtError reports whether what MatchError is given is an error the spec
// builds itself, which MatchError compares with reflect.DeepEqual: by its
// text, in practice.
func (f *fileChecker) builtError(expr ast.Expr) bool {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		return false
	}
	if name, found := f.refers(call.Fun, errorsPath); found {
		return name == "New"
	}
	name, found := f.refers(call.Fun, fmtPath)
	return found && name == "Errorf"
}

// positiveText reports whether what MatchError is given asserts text
// in positive polarity.
func (f *fileChecker) positiveText(expr ast.Expr, negated bool) bool {
	switch e := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		return !negated && e.Kind == token.STRING
	case *ast.BinaryExpr:
		if holdsLiteral(e) {
			return !negated
		}
	case *ast.CallExpr:
		if _, ok := f.errorText(e); ok {
			return !negated
		}
		if name, ok := f.refers(e.Fun, fmtPath); ok && slices.Contains(sprints, name) {
			return !negated
		}
		if f.stringTransformation(e.Fun) {
			return !negated
		}
		name, ok := f.refers(e.Fun, gomegaPath)
		switch {
		case !ok:
		case name == "Not" && len(e.Args) == 1:
			return f.positiveText(e.Args[0], !negated)
		case slices.Contains(compositions, name):
			return slices.ContainsFunc(e.Args, func(arg ast.Expr) bool { return f.positiveText(arg, negated) })
		case name == "WithTransform" && len(e.Args) == 2:
			// A strings transformation leaves text text; any other makes something
			// else of it.
			return f.stringTransformation(e.Args[0]) && f.positiveText(e.Args[1], negated)
		case slices.Contains(stringMatchers, name):
			return !negated
		}
	}
	// A variable holding an error's text; a string variable no write makes text
	// is not judged.
	if _, ok := f.textOf(expr); ok {
		return !negated
	}
	return false
}

// positiveLeaf reports whether a matcher applies anything in positive polarity:
// itself, or one of the matchers it composes, under every Not enclosing it.
// A WithTransform whose transform is not a strings transformation applies its
// matcher to whatever the transform made of the value, so it applies nothing
// the rules judge.
func (f *fileChecker) positiveLeaf(matcher ast.Expr, negated bool) bool {
	if call, ok := ast.Unparen(matcher).(*ast.CallExpr); ok {
		name, ok := f.refers(call.Fun, gomegaPath)
		switch {
		case ok && name == "Not" && len(call.Args) == 1:
			return f.positiveLeaf(call.Args[0], !negated)
		case ok && slices.Contains(compositions, name):
			return slices.ContainsFunc(call.Args, func(arg ast.Expr) bool { return f.positiveLeaf(arg, negated) })
		case ok && name == "WithTransform" && len(call.Args) == 2:
			return f.stringTransformation(call.Args[0]) && f.positiveLeaf(call.Args[1], negated)
		}
	}
	return !negated
}

// holdsLiteral reports whether a string concatenation holds a string literal.
func holdsLiteral(expr ast.Expr) bool {
	switch e := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.BinaryExpr:
		return e.Op == token.ADD && (holdsLiteral(e.X) || holdsLiteral(e.Y))
	}
	return false
}
