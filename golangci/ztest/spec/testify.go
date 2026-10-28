package spec

import (
	"go/ast"
	"strings"
)

// testifyValues maps each testify assertion to how many values under test
// it takes, past the testing.T a package function takes first. What follows
// them is the failure message and its arguments, which assert nothing.
// An assertion's f form takes the same values, then a format and its arguments.
var testifyValues = map[string]int{
	"Condition": 1, "Contains": 2, "DirExists": 1, "ElementsMatch": 2, "Empty": 1, "Equal": 2,
	"EqualError": 2, "EqualExportedValues": 2, "EqualValues": 2, "Error": 1, "ErrorAs": 2, "ErrorContains": 2,
	"ErrorIs": 2, "Eventually": 3, "EventuallyWithT": 3, "Exactly": 2, "Fail": 1, "FailNow": 1, "False": 1,
	"FileExists": 1, "Greater": 2, "GreaterOrEqual": 2, "HTTPBodyContains": 5, "HTTPBodyNotContains": 5,
	"HTTPError": 4, "HTTPRedirect": 4, "HTTPStatusCode": 5, "HTTPSuccess": 4, "Implements": 2, "InDelta": 3,
	"InDeltaMapValues": 3, "InDeltaSlice": 3, "InEpsilon": 3, "InEpsilonSlice": 3, "IsDecreasing": 1,
	"IsIncreasing": 1, "IsNonDecreasing": 1, "IsNonIncreasing": 1, "IsNotType": 2, "IsType": 2, "JSONEq": 2,
	"Len": 2, "Less": 2, "LessOrEqual": 2, "Negative": 1, "Never": 3, "Nil": 1, "NoDirExists": 1, "NoError": 1,
	"NoFileExists": 1, "NotContains": 2, "NotElementsMatch": 2, "NotEmpty": 1, "NotEqual": 2, "NotEqualValues": 2,
	"NotErrorAs": 2, "NotErrorIs": 2, "NotImplements": 2, "NotNil": 1, "NotPanics": 1, "NotRegexp": 2,
	"NotSame": 2, "NotSubset": 2, "NotZero": 1, "Panics": 1, "PanicsWithError": 2, "PanicsWithValue": 2,
	"Positive": 1, "Regexp": 2, "Same": 2, "Subset": 2, "True": 1, "WithinDuration": 3, "WithinRange": 3,
	"YAMLEq": 2, "Zero": 1,
}

// testify judges a testify assertion, made through the assert or require
// package or through their *Assertions, a testify suite's receiver included.
func (f *fileChecker) testify(call *ast.CallExpr) {
	written, name, values, ok := f.testifyAssertion(call)
	if !ok {
		return
	}
	f.testifyIdentity(call, name, values)
	f.testifyPin(name, values)
	switch name {
	case "Error":
		f.unidentified(call.Pos(), written+" says only that some error occurred; "+
			"assert which one, with ErrorIs, ErrorAs, or its type and fields")
	case "NotNil":
		if len(values) > 0 && f.isError(values[0]) {
			f.unidentified(call.Pos(), written+" of an error says only that some error occurred; "+
				"assert which one, with ErrorIs, ErrorAs, or its type and fields")
		}
	case "EqualError", "ErrorContains":
		if len(values) > 0 {
			f.textAsserted(call.Pos(), textMessage, values[0])
		}
	default:
		// A Not assertion proves text absent, which only a text assertion can.
		if strings.HasPrefix(name, "Not") {
			return
		}
		var subjects []ast.Expr
		var enums []enumRef
		found := false
		for _, value := range values {
			if text, ok := f.textOf(value); ok {
				subjects = append(subjects, text...)
				found = true
			}
			enums = append(enums, f.renderingOf(value)...)
		}
		if found {
			f.textAsserted(call.Pos(), textMessage, subjects...)
		}
		if len(enums) > 0 {
			f.tokenAsserted(call.Pos(), enums)
		}
	}
}

// testifyAssertion returns the testify assertion a call makes, as written
// and without its f suffix, and the values under test it is given: past
// the testing.T a package function takes first. A name testify does not declare
// as an assertion is none.
func (f *fileChecker) testifyAssertion(call *ast.CallExpr) (string, string, []ast.Expr, bool) {
	fn := f.funcOf(call.Fun)
	if !testify(fn) {
		return "", "", nil, false
	}
	name, args := fn.Name(), call.Args
	if fn.Signature().Recv() == nil {
		if len(args) == 0 {
			return "", "", nil, false
		}
		args = args[1:]
	}

	base := name
	count, ok := testifyValues[name]
	if !ok {
		var isF bool
		base, isF = strings.CutSuffix(name, "f")
		if count, ok = testifyValues[base]; !ok || !isF {
			return "", "", nil, false
		}
	}
	return name, base, args[:min(count, len(args))], true
}
