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

// numericVerbs are the verbs under which fmt prints an integer's digits
// or the rune it encodes rather than calling its String. %x and %X do call
// a String, but print its bytes in hex, which is no member's name either.
const numericVerbs = "dxXoObcU"

// naming are the verbs under which fmt calls a value's String, where it can.
const naming = "vsq"

// formatting judges what a printer's call formats: each operand under the verb
// that formats it, as a printf printer's constant format pairs them, or as %v
// for a print printer. Only operands written out are judged, not ones spread
// from a slice, and a printf printer's format is read only when it is constant.
// An operand whose type holds a type parameter is judged again in each
// instantiation the module makes.
//
// own is the enum whose method the call sits in, if any. Its values are not
// judged there: its String printing a value no member holds as a number,
// as in "Kind(%d)", is the one place the ordinal is what should print, and %v
// would call that String again.
func (a *analysis) formatting(info *types.Info, call *ast.CallExpr, own *enumType) {
	callee := typeutil.Callee(info, call)
	if callee == nil || call.Ellipsis.IsValid() {
		return
	}
	p, ok := a.printerOf(callee)
	if !ok {
		return
	}
	operands := formattedOperands(info, call, p, signatureOf(callee))
	var generic []formatted
	for _, operand := range operands {
		t := info.TypeOf(operand.expr)
		a.judgeFormatted(operand, t, own)
		if holdsTypeParam(t) {
			generic = append(generic, operand)
		}
	}
	if len(generic) > 0 {
		a.genericFormats[call.Lparen] = genericFormat{operands: generic, own: own}
	}
}

// formatted is one operand a printer's call formats, and the verb
// it is formatted under.
type formatted struct {
	expr ast.Expr

	// index is the operand's position among the call's variadic operands.
	index int

	verb  rune
	sharp bool
}

// formattedOperands pairs each operand of a printer's call with the verb
// that formats it.
func formattedOperands(info *types.Info, call *ast.CallExpr, p printer, sig *types.Signature) []formatted {
	variadic := sig.Params().Len() - 1
	if len(call.Args) <= variadic {
		return nil
	}
	operands := call.Args[variadic:]
	if !p.printf() {
		found := make([]formatted, len(operands))
		for i, operand := range operands {
			found[i] = formatted{expr: operand, index: i, verb: 'v'}
		}
		return found
	}
	format := info.Types[call.Args[p.format]].Value
	if format == nil || format.Kind() != constant.String {
		return nil
	}
	var found []formatted
	for _, d := range directives(constant.StringVal(format), len(operands)) {
		found = append(found, formatted{expr: operands[d.operand], index: d.operand, verb: d.verb, sharp: d.sharp})
	}
	return found
}

// judgeFormatted reports what fmt prints of an operand of type t that is no
// member's name.
func (a *analysis) judgeFormatted(operand formatted, t types.Type, own *enumType) {
	pos := operand.expr.Pos()
	numeric := strings.ContainsRune(numericVerbs, operand.verb)
	named := strings.ContainsRune(naming, operand.verb) && !operand.sharp
	if !numeric && !named {
		return
	}
	for _, reached := range a.reach(t, operand.verb) {
		if reached.enum == own {
			continue
		}
		switch {
		case numeric:
			a.report(pos, RuleNumericVerb, reached.enum, fmt.Sprintf(
				"formats %s under %%%c, which prints its ordinal rather than its member's name; use %%v or %%s",
				reached.enum.display(), operand.verb))
		case reached.bytes != nil && operand.verb != 'v':
			a.report(pos, RuleStringBypass, reached.enum, fmt.Sprintf(
				"formats %s as the elements of %s under %%%c, which fmt prints as raw bytes; format it with %%v",
				reached.enum.display(), types.TypeString(reached.bytes, a.qualifier), operand.verb))
		case reached.field != "":
			a.report(pos, RuleStringBypass, reached.enum, fmt.Sprintf(
				"formats %s beneath the unexported field %s, where fmt calls no String; "+
					"export the field, or format the field itself",
				reached.enum.display(), reached.field))
		}
	}
}

// qualifier writes a type's package by its name, as source code spells it.
func (a *analysis) qualifier(pkg *types.Package) string {
	return pkg.Name()
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
// sharp is the # flag, which asks for Go syntax under %v.
type directive struct {
	verb    rune
	operand int
	sharp   bool
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
		sharp := false
		for p.i < len(format) && strings.IndexByte("#0+- ", format[p.i]) >= 0 {
			sharp = sharp || format[p.i] == '#'
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
		found = append(found, directive{verb: verb, operand: p.arg, sharp: sharp})
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
