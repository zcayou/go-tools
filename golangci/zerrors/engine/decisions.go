package engine

import (
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/zcayou/go-tools/golangci/internal/decision"
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// textDecisions reports each text decision module code makes on an error's
// text: the result of Error(), fmt.Sprint or fmt.Sprintf of an error,
// or a string field of a module error type. A generic body and its instances
// make the same decision at the same place, which is reported once.
func (a *analysis) textDecisions() []Finding {
	var findings []Finding
	seen := map[token.Pos]bool{}
	for _, fn := range a.idx.funcs {
		for _, d := range decision.Find(fn, a.errorText) {
			if seen[d.Pos] {
				continue
			}
			seen[d.Pos] = true
			findings = append(findings, Finding{
				Pos:     a.module.Fset.PositionFor(d.Pos, false),
				Rule:    RuleTextDecision,
				Subject: placeName(declaringFunction(fn)),
				Message: decisionMessage(d.Subject),
			})
		}
	}
	return findings
}

// decisionMessage says what a decision on subject should branch on instead.
// A decision on an error's text points at its type or fields; one on a string
// field of an error type points at the field, which is the enum the decision
// reads in a type that does not name it.
func decisionMessage(subject ssa.Value) string {
	var holder types.Type
	var index int
	switch subject := subject.(type) {
	case *ssa.UnOp:
		addr, ok := subject.X.(*ssa.FieldAddr)
		if !ok {
			return errorTextMessage
		}
		holder, index = addr.X.Type(), addr.Field
	case *ssa.Field:
		holder, index = subject.X.Type(), subject.Field
	default:
		return errorTextMessage
	}
	return "decides on the text of " + fieldName(holder, fieldOf(holder, index)) +
		"; give the field an enum type and branch on its members"
}

// errorTextMessage is what a decision on an error's own text says.
const errorTextMessage = "decides on an error's text; branch on its type or fields instead"

// declaringFunction is the declared function a body comes from.
func declaringFunction(fn *ssa.Function) *ssa.Function {
	top := topLevel(fn)
	if origin := top.Origin(); origin != nil {
		return origin
	}
	return top
}

// errorText reports whether v is an error's text.
func (a *analysis) errorText(v ssa.Value) bool {
	switch v := v.(type) {
	case *ssa.Call:
		return errorCall(&v.Call)
	case *ssa.UnOp:
		addr, ok := v.X.(*ssa.FieldAddr)
		return ok && v.Op == token.MUL && a.errorField(addr.X.Type(), addr.Field, addr.X)
	case *ssa.Field:
		return a.errorField(v.X.Type(), v.Field, v.X)
	}
	return false
}

// errorCall reports whether a call renders an error: its Error method,
// or fmt.Sprint or fmt.Sprintf handed one.
func errorCall(call *ssa.CallCommon) bool {
	if call.IsInvoke() {
		return errorMethod(call.Method)
	}
	callee := call.StaticCallee()
	if callee == nil {
		return false
	}
	obj, ok := callee.Object().(*types.Func)
	if !ok {
		return false
	}
	if obj.Signature().Recv() != nil {
		return errorMethod(obj) && errorNamed(obj.Signature().Recv().Type()) != nil
	}
	if obj.Pkg() == nil || obj.Pkg().Path() != "fmt" || obj.Name() != "Sprint" && obj.Name() != "Sprintf" {
		return false
	}
	args, ok := wholemodule.VariadicArgs(call)
	if !ok {
		return false
	}
	for _, arg := range args {
		switch arg := arg.(type) {
		case *ssa.MakeInterface:
			if errorNamed(arg.X.Type()) != nil {
				return true
			}
		case *ssa.ChangeInterface:
			if errorValued(arg.X.Type()) {
				return true
			}
		}
	}
	return false
}

func errorMethod(fn *types.Func) bool {
	sig := fn.Signature()
	return fn.Name() == "Error" && sig.Params().Len() == 0 && sig.Results().Len() == 1 &&
		types.Identical(sig.Results().At(0).Type(), types.Typ[types.String])
}

// errorField reports whether field i of a struct of type holder, reached
// through x, is a string field of a module error type: one the error type
// declares, or one it holds through a chain of embedded fields.
func (a *analysis) errorField(holder types.Type, i int, x ssa.Value) bool {
	if !types.Identical(fieldOf(holder, i).Type(), types.Typ[types.String]) {
		return false
	}
	for {
		if named := errorNamed(holder); named != nil && a.module.Declares(named.Obj()) {
			return true
		}
		switch embedding := x.(type) {
		case *ssa.FieldAddr:
			if !fieldOf(embedding.X.Type(), embedding.Field).Embedded() {
				return false
			}
			holder, x = embedding.X.Type(), embedding.X
		case *ssa.Field:
			if !fieldOf(embedding.X.Type(), embedding.Field).Embedded() {
				return false
			}
			holder, x = embedding.X.Type(), embedding.X
		default:
			return false
		}
	}
}
