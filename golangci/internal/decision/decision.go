// Package decision recognizes text decisions: branches taken on the content
// of a string the caller cares about, its subject.
//
// What makes a string a subject is the caller's business — an error's text
// for one linter, an enum member's rendered name for another — and is handed
// in as a predicate over SSA values. What makes a use of it a decision
// is shared, so the word means the same thing in every linter:
//
//   - a call to a strings matcher, Contains*, HasPrefix, HasSuffix, Index*,
//     LastIndex*, Cut*, EqualFold or Count, with the subject as one operand
//     and a constant as another: a non-empty string, or a rune or byte;
//   - a regexp match against the subject;
//   - == or != between the subject and a non-empty constant, which is also
//     what a case of a switch on the subject compiles to.
//
// Comparing with "" is a presence check, and comparing one subject with another
// decides nothing about either's content. A matcher between a subject
// and a computed string is that same comparison, such as EqualFold of a token
// and the input being parsed into one, and matching "" decides nothing.
// Ordering by text, through strings.Compare, cmp.Compare or a sort, is not
// a branch on content. None of them is a decision.
//
// A value derived from a subject is a subject too: a strings transformation
// of it, such as ToLower or TrimSpace, a slice of it, and a local any
// assignment of which gives it a subject.
package decision

import (
	"go/constant"
	"go/token"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// Subject reports whether a value is a string whose content the caller judges
// decisions on.
type Subject func(ssa.Value) bool

// Decision is one text decision on a subject.
type Decision struct {
	// Pos is where the decision is made: the comparison's operator, the case
	// a switch compares against, or the opening parenthesis of the matching call.
	Pos token.Pos

	// Subject is the value decided on, as the caller's predicate recognized it,
	// before any transformation.
	Subject ssa.Value
}

// Find returns the text decisions fn makes on subjects, in instruction order.
// It reads fn's own body only: a function literal inside it is a function
// of its own, in fn.AnonFuncs, which the caller passes separately.
func Find(fn *ssa.Function, subject Subject) []Decision {
	d := &deriver{subject: subject, memo: map[ssa.Value]ssa.Value{}}
	var found []Decision
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			switch instr := instr.(type) {
			case *ssa.BinOp:
				if instr.Op != token.EQL && instr.Op != token.NEQ {
					continue
				}
				if root := d.compared(instr.X, instr.Y); root != nil {
					found = append(found, Decision{Pos: instr.Pos(), Subject: root})
				} else if root := d.compared(instr.Y, instr.X); root != nil {
					found = append(found, Decision{Pos: instr.Pos(), Subject: root})
				}
			case ssa.CallInstruction:
				if root := d.matched(instr.Common()); root != nil {
					found = append(found, Decision{Pos: instr.Pos(), Subject: root})
				}
			}
		}
	}
	return found
}

// compared returns the subject x derives from when y is a non-empty string
// constant.
func (d *deriver) compared(x, y ssa.Value) ssa.Value {
	c, ok := y.(*ssa.Const)
	if !ok || c.Value == nil || c.Value.Kind() != constant.String || constant.StringVal(c.Value) == "" {
		return nil
	}
	return d.root(x)
}

// matched returns the subject a call matches text against, or nil when
// it matches none. A regexp match against a subject always decides on it.
// A strings matcher decides only against a constant, as == does: one of its
// other operands is a non-empty string constant, or a constant rune or byte.
func (d *deriver) matched(call *ssa.CallCommon) ssa.Value {
	switch matcher(call) {
	case matchNone:
		return nil
	case matchRegexp:
		for _, arg := range call.Args {
			if root := d.root(arg); root != nil {
				return root
			}
		}
	case matchStrings:
		for i, arg := range call.Args {
			root := d.root(arg)
			if root == nil {
				continue
			}
			for j, other := range call.Args {
				if j != i && textConstant(other) {
					return root
				}
			}
		}
	}
	return nil
}

// textConstant reports whether v is a constant a strings matcher can match
// text against: a non-empty string, or a rune or byte.
func textConstant(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	if !ok || c.Value == nil {
		return false
	}
	switch c.Value.Kind() {
	case constant.String:
		return constant.StringVal(c.Value) != ""
	case constant.Int:
		return true
	default:
		return false
	}
}

// match is the kind of text matching a call performs.
type match int

const (
	// matchNone is a call that matches no text.
	matchNone match = iota + 1
	// matchStrings is a strings matcher, which compares operands with each other.
	matchStrings
	// matchRegexp is a regexp match, whose pattern is text of its own.
	matchRegexp
)

// matcher reports how a call matches text.
func matcher(call *ssa.CallCommon) match {
	fn := call.StaticCallee()
	if fn == nil || fn.Object() == nil || fn.Object().Pkg() == nil {
		return matchNone
	}
	name := fn.Object().Name()
	switch fn.Object().Pkg().Path() {
	case "strings":
		if fn.Signature.Recv() != nil {
			return matchNone
		}
		switch name {
		case "HasPrefix", "HasSuffix", "EqualFold", "Count":
			return matchStrings
		}
		for _, prefix := range []string{"Contains", "Index", "LastIndex", "Cut"} {
			if strings.HasPrefix(name, prefix) {
				return matchStrings
			}
		}
		return matchNone
	case "regexp":
		if strings.HasPrefix(name, "Match") || strings.HasPrefix(name, "Find") {
			return matchRegexp
		}
		return matchNone
	default:
		return matchNone
	}
}

// transforms are the strings functions whose result is their first operand
// transformed, and so carries its content.
var transforms = map[string]bool{
	"Clone": true, "Replace": true, "ReplaceAll": true, "Title": true, "ToLower": true, "ToTitle": true,
	"ToUpper": true, "ToValidUTF8": true, "Trim": true, "TrimFunc": true, "TrimLeft": true,
	"TrimLeftFunc": true, "TrimPrefix": true, "TrimRight": true, "TrimRightFunc": true, "TrimSpace": true,
	"TrimSuffix": true,
}

// deriver finds the subject a value derives from.
type deriver struct {
	subject Subject

	// memo holds each value's subject, or nil for one that derives from none.
	// A value being derived is held as nil until it is settled, which keeps
	// a loop through a phi from recursing forever.
	memo map[ssa.Value]ssa.Value
}

func (d *deriver) root(v ssa.Value) ssa.Value {
	if root, ok := d.memo[v]; ok {
		return root
	}
	d.memo[v] = nil
	root := d.derive(v)
	d.memo[v] = root
	return root
}

func (d *deriver) derive(v ssa.Value) ssa.Value {
	if d.subject(v) {
		return v
	}
	switch v := v.(type) {
	case *ssa.Phi:
		for _, edge := range v.Edges {
			if root := d.root(edge); root != nil {
				return root
			}
		}
	case *ssa.Slice:
		return d.root(v.X)
	case *ssa.ChangeType:
		return d.root(v.X)
	case *ssa.Call:
		if arg, ok := transformed(&v.Call); ok {
			return d.root(arg)
		}
	case *ssa.UnOp:
		// A load of a local that escapes into the heap, as one a closure captures
		// does: a store of a subject into it makes it one.
		if alloc, ok := v.X.(*ssa.Alloc); ok && v.Op == token.MUL {
			return d.stored(alloc)
		}
	}
	return nil
}

// transformed returns the operand a strings transformation carries the content
// of, or false for any other call.
func transformed(call *ssa.CallCommon) (ssa.Value, bool) {
	fn := call.StaticCallee()
	if fn == nil || fn.Object() == nil || fn.Object().Pkg() == nil || fn.Signature.Recv() != nil {
		return nil, false
	}
	if fn.Object().Pkg().Path() != "strings" || !transforms[fn.Object().Name()] || len(call.Args) == 0 {
		return nil, false
	}
	return call.Args[0], true
}

// stored returns the subject any store into a local derives from.
func (d *deriver) stored(alloc *ssa.Alloc) ssa.Value {
	for _, ref := range *alloc.Referrers() {
		if store, ok := ref.(*ssa.Store); ok && store.Addr == alloc {
			if root := d.root(store.Val); root != nil {
				return root
			}
		}
	}
	return nil
}
