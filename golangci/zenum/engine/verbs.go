package engine

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/go/types/typeutil"
)

// bypassing are the verbs under which fmt prints an integer's digits
// or the rune it encodes rather than calling its String. %x and %X do call
// a String, but print its bytes in hex, which is no member's name either.
const bypassing = "dxXobcU"

// formatting reports an ordinal enum value formatted by a fmt-style call
// under a verb that bypasses its String. A fmt-style call is one to a function
// or method whose name ends in f and whose last two parameters are a format
// string and ...any, which covers fmt's own, log's, and wrappers written
// in their image. Only a constant format can be read, and only operands
// written out, not spread from a slice.
//
// own is the enum whose method the call sits in, if any. Its values are not
// judged there: its String printing a value no member holds as a number,
// as in "Kind(%d)", is the one place the ordinal is what should print, and %v
// would call that String again.
func (a *analysis) formatting(info *types.Info, call *ast.CallExpr, own *enumType) {
	callee, ok := typeutil.Callee(info, call).(*types.Func)
	if !ok || !strings.HasSuffix(callee.Name(), "f") || call.Ellipsis.IsValid() {
		return
	}
	at, ok := formatParam(callee.Signature())
	if !ok || at >= len(call.Args) {
		return
	}
	format := info.Types[call.Args[at]].Value
	if format == nil || format.Kind() != constant.String {
		return
	}
	operands := call.Args[at+1:]
	for _, directive := range directives(constant.StringVal(format), len(operands)) {
		if !strings.ContainsRune(bypassing, directive.verb) {
			continue
		}
		operand := operands[directive.operand]
		v := a.enumOf(info.TypeOf(operand))
		if v == nil || v == own || !v.Ordinal() {
			continue
		}
		a.report(operand.Pos(), RuleNumericVerb, v, fmt.Sprintf(
			"formats %s under %%%c, which prints its ordinal rather than its member's name; use %%v or %%s",
			v.display(), directive.verb))
	}
}

// formatParam returns the index of a fmt-style signature's format parameter:
// a string followed by ...any, ending the parameters.
func formatParam(sig *types.Signature) (int, bool) {
	params := sig.Params()
	n := params.Len()
	if !sig.Variadic() || n < 2 || !types.Identical(params.At(n-2).Type(), types.Typ[types.String]) {
		return 0, false
	}
	slice, ok := params.At(n - 1).Type().(*types.Slice)
	if !ok {
		return 0, false
	}
	iface, ok := slice.Elem().Underlying().(*types.Interface)
	return n - 2, ok && iface.Empty()
}

// directive is one verb of a format string, and the operand it formats.
type directive struct {
	verb    rune
	operand int
}

// directives pairs each verb of a format string with the operand it formats,
// as fmt's doPrintf does for a call given that many operands: flags, then
// an optional explicit operand index, width and precision, each width
// or precision written as * consuming an operand of its own. A verb fmt prints
// as a bad or missing operand formats none: one after a malformed index, such
// as [0], [] or [x], or an index out of range, and one past the last operand.
// So does %%.
func directives(format string, operands int) []directive {
	p := &formatParser{format: format, operands: operands}
	var found []directive
	for p.i < len(format) {
		if format[p.i] != '%' {
			p.i++
			continue
		}
		p.i++
		p.good = true
		for p.i < len(format) && strings.IndexByte("#0+- ", format[p.i]) >= 0 {
			p.i++
		}
		afterIndex := p.argIndex()
		if p.peek('*') {
			p.i++
			p.star()
			afterIndex = false
		} else if p.digits() && afterIndex {
			p.good = false // %[3]2d
		}
		if p.i+1 < len(format) && format[p.i] == '.' {
			p.i++
			if afterIndex {
				p.good = false // %[3].2d
			}
			afterIndex = p.argIndex()
			if p.peek('*') {
				p.i++
				p.star()
				afterIndex = false
			} else {
				p.digits()
			}
		}
		if !afterIndex {
			p.argIndex()
		}
		if p.i >= len(format) {
			break
		}
		verb, size := utf8.DecodeRuneInString(format[p.i:])
		p.i += size
		if verb == '%' || !p.good || p.arg >= operands {
			continue
		}
		found = append(found, directive{verb: verb, operand: p.arg})
		p.arg++
	}
	return found
}

// formatParser is a position in a format string, and the operand the next verb
// formats.
type formatParser struct {
	format   string
	operands int
	i        int
	arg      int
	// good is false once the current verb's operand index went bad.
	good bool
}

func (p *formatParser) peek(c byte) bool {
	return p.i < len(p.format) && p.format[p.i] == c
}

// argIndex reads an explicit operand index, [n], which makes the next operand
// the nth. It reports whether one was read. A malformed or out-of-range index
// is consumed and leaves the operand where it was, with the verb gone bad.
func (p *formatParser) argIndex() bool {
	if !p.peek('[') {
		return false
	}
	closing := strings.IndexByte(p.format[p.i:], ']')
	if len(p.format)-p.i < 3 || closing < 0 {
		p.i++
		p.good = false
		return false
	}
	n, ok := number(p.format[p.i+1 : p.i+closing])
	p.i += closing + 1
	if !ok || n < 1 || n > p.operands {
		p.good = false
		return ok
	}
	p.arg = n - 1
	return true
}

// star consumes the operand a * width or precision takes, when there is one.
func (p *formatParser) star() {
	if p.arg < p.operands {
		p.arg++
	}
}

// digits skips a written width or precision, reporting whether there was one.
func (p *formatParser) digits() bool {
	start := p.i
	for p.i < len(p.format) && p.format[p.i] >= '0' && p.format[p.i] <= '9' {
		p.i++
	}
	return p.i > start
}

// number parses a decimal operand index as fmt does, refusing an empty one
// and one too large to be any call's.
func number(text string) (int, bool) {
	if text == "" {
		return 0, false
	}
	n := 0
	for _, digit := range text {
		if digit < '0' || digit > '9' || n > 1e6 {
			return 0, false
		}
		n = n*10 + int(digit-'0')
	}
	return n, true
}
