package engine

import (
	"cmp"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// maxConjunctions bounds the conditions a wrapping carries. Each
// is a conjunction of the parameters and free variables that, all structured,
// make the wrapping structured; combining operands multiplies them, and past
// the bound a wrapping is judged by what it holds whatever its arguments.
const maxConjunctions = 8

// witness is one unstructured origin, as a finding names it.
type witness struct {
	// what names the origin: fmt.Errorf, io.EOF, a value of some type.
	what string

	// where is the declared function the origin sits in, a closure's being
	// the function it is nested in.
	where *ssa.Function

	// initializes is the package-level variable whose initializer the origin
	// sits in, or empty.
	initializes string

	// position is where the origin sits, resolved, which orders witnesses
	// the same way in every run: token.Pos order follows the order the loader
	// happened to parse files in.
	position token.Position
}

// compare orders witnesses, so that which one a finding names depends neither
// on the order the analysis met them in nor on the order files were parsed in.
func (w *witness) compare(other *witness) int {
	return cmp.Or(
		cmp.Compare(w.position.Filename, other.position.Filename),
		cmp.Compare(w.position.Offset, other.position.Offset),
		strings.Compare(w.what, other.what),
		strings.Compare(w.where.String(), other.where.String()),
		strings.Compare(w.initializes, other.initializes),
	)
}

// dep is a parameter, or a free variable of a closure, whose value a call
// supplies.
type dep struct {
	// free marks a free variable rather than a parameter.
	free bool

	// index is the parameter's or the free variable's position.
	index int
}

func (d dep) compare(other dep) int {
	if d.free != other.free {
		if d.free {
			return 1
		}
		return -1
	}
	return cmp.Compare(d.index, other.index)
}

// conjunction is a set of deps, sorted, that are structured together.
type conjunction []dep

func (c conjunction) compare(other conjunction) int {
	return slices.CompareFunc(c, other, dep.compare)
}

// wrap is a call wrapping its operands, fmt.Errorf with %w or errors.Join,
// that is unstructured unless the deps of one of its rescues are all
// structured.
type wrap struct {
	call *witness

	// judged reports that, rescued by none, the wrapping is unstructured only
	// through a judged function's error.
	judged bool

	// rescues are the conditions under which the wrapping is structured,
	// sorted. A wrap with none is no longer conditional, and is never held.
	rescues []conjunction
}

// origins is what an error value can be, as far as unstructured-return judges
// it. The zero value can be nothing at all, as a value still being traced
// around a loop adds nothing to what the loop's head reaches.
type origins struct {
	// produced is an unstructured origin the value reaches without passing
	// through a call to a judged function.
	produced *witness

	// viaJudged reports that the value can be unstructured through a call
	// to a judged function, which carries the finding itself.
	viaJudged bool

	// structured reports that the value can be a structured error, or one
	// the rule does not judge.
	structured bool

	// nilable reports that the value can be nil.
	nilable bool

	// neutral reports that the value can be what a cause slot holds: an error
	// judged where it was made, which neither is a defect here nor makes
	// a wrapping of it structured.
	neutral bool

	// deps are the parameters and free variables, sorted, the value can be:
	// what it is then depends on the call.
	deps []dep

	// wraps are the wrappings, sorted by their call, the value can be.
	wraps []wrap
}

// structuredOrigin is a value that can only be structured, or is not judged.
func structuredOrigin() origins {
	return origins{structured: true}
}

// reported reports whether a judged function's result can be unstructured
// by itself. Its parameters are its caller's own errors, so every wrapping
// conditional on them is structured.
func (o origins) reported() bool {
	return o.produced != nil || o.viaJudged
}

// condition returns the rescues under which the value structures a wrapping
// of it, a single empty conjunction meaning always, or false when no arguments
// make it so. A value that can be unstructured never does. Nil and a cause
// slot's value count for nothing either way: a value that can be either
// or structured is structured wherever it is wrapped, as it is behind a check
// that it is not nil, while one that can only be either structures nothing.
func (o origins) condition() ([]conjunction, bool) {
	if o.produced != nil || o.viaJudged || !o.structured && len(o.deps) == 0 && len(o.wraps) == 0 {
		return nil, false
	}
	rescues := []conjunction{slices.Clone(o.deps)}
	for _, w := range o.wraps {
		var ok bool
		if rescues, ok = cross(rescues, w.rescues); !ok {
			return nil, false
		}
	}
	return rescues, true
}

// cross returns every union of one conjunction of each, or false past
// maxConjunctions.
func cross(left, right []conjunction) ([]conjunction, bool) {
	if len(left)*len(right) > maxConjunctions {
		return nil, false
	}
	var out []conjunction
	for _, l := range left {
		for _, r := range right {
			out = append(out, unionDeps(l, r))
		}
	}
	return normalize(out), true
}

// normalize sorts conjunctions and drops duplicates.
func normalize(rescues []conjunction) []conjunction {
	slices.SortFunc(rescues, conjunction.compare)
	return slices.CompactFunc(rescues, func(x, y conjunction) bool { return x.compare(y) == 0 })
}

func (o origins) join(other origins) origins {
	out := origins{
		produced:   o.produced,
		viaJudged:  o.viaJudged || other.viaJudged,
		structured: o.structured || other.structured,
		nilable:    o.nilable || other.nilable,
		neutral:    o.neutral || other.neutral,
		deps:       unionDeps(o.deps, other.deps),
		wraps:      joinWraps(o.wraps, other.wraps),
	}
	if other.produced != nil && (out.produced == nil || other.produced.compare(out.produced) < 0) {
		out.produced = other.produced
	}
	if !slices.ContainsFunc(out.wraps, func(w wrap) bool { return len(w.rescues) == 0 }) {
		return out
	}
	// A call two paths rescue under no common condition is rescued by none.
	held := out.wraps[:0:0]
	for _, w := range out.wraps {
		if len(w.rescues) > 0 {
			held = append(held, w)
			continue
		}
		settled := unconditional(w.call, w.judged)
		out.viaJudged = out.viaJudged || settled.viaJudged
		if settled.produced != nil && (out.produced == nil || settled.produced.compare(out.produced) < 0) {
			out.produced = settled.produced
		}
	}
	out.wraps = held
	return out
}

func (o origins) equal(other origins) bool {
	samePlace := o.produced == other.produced ||
		o.produced != nil && other.produced != nil && o.produced.compare(other.produced) == 0
	return samePlace && o.viaJudged == other.viaJudged && o.structured == other.structured &&
		o.nilable == other.nilable &&
		o.neutral == other.neutral && slices.Equal(o.deps, other.deps) &&
		slices.EqualFunc(o.wraps, other.wraps, func(x, y wrap) bool {
			return x.call.compare(y.call) == 0 && x.judged == y.judged &&
				slices.EqualFunc(x.rescues, y.rescues, func(a, b conjunction) bool { return a.compare(b) == 0 })
		})
}

func unionDeps(a, b []dep) []dep {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	out := slices.Concat(a, b)
	slices.SortFunc(out, dep.compare)
	return slices.Compact(out)
}

// joinWraps unions two sets of wrappings. One call met along two paths keeps
// only the rescues both allow, so that what can be unstructured only grows.
func joinWraps(a, b []wrap) []wrap {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	out := slices.Clone(a)
	for _, w := range b {
		i := slices.IndexFunc(out, func(x wrap) bool { return x.call.compare(w.call) == 0 })
		if i < 0 {
			out = append(out, w)
			continue
		}
		rescues := slices.DeleteFunc(slices.Clone(out[i].rescues), func(c conjunction) bool {
			return !slices.ContainsFunc(w.rescues, func(d conjunction) bool { return c.compare(d) == 0 })
		})
		out[i] = wrap{call: out[i].call, judged: out[i].judged && w.judged, rescues: rescues}
	}
	slices.SortFunc(out, func(x, y wrap) int { return x.call.compare(y.call) })
	return out
}

// wrapping is what a call wrapping its operands makes, fmt.Errorf with %w
// or errors.Join, given what each operand can be.
//
// It is structured as soon as one operand structures it: one that can
// be a structured error and cannot be an unstructured one, nil and a cause
// slot's value counting for nothing. An operand structuring it only under
// some arguments makes the wrapping conditional on them. A wrapping no operand
// can make unstructured is what its operands can be: errors.Join drops its nil
// operands, so one whose every operand can be nil can be nil, and either
// wrapping of a cause slot's value is as neutral as the value. Otherwise
// the wrapping is unstructured, the call itself being the origin named unless
// only a judged function's error made it so; fmt.Errorf wrapping only nil
// makes an error of its own.
func wrapping(operands []origins, call *witness, join bool) origins {
	var rescues []conjunction
	judged, unstructured, neutral, nilable := true, false, false, join
	for _, operand := range operands {
		conditions, ok := operand.condition()
		if ok && len(conditions) == 1 && len(conditions[0]) == 0 {
			return structuredOrigin()
		}
		if ok {
			rescues = append(rescues, conditions...)
		}
		neutral = neutral || operand.neutral
		nilable = nilable && operand.nilable
		if operand.unsettled() || operand.viaJudged {
			unstructured = true
		}
		if operand.unsettled() || operand.nilable && !join {
			judged = false
		}
	}
	switch {
	case !unstructured && join:
		return origins{neutral: neutral, nilable: nilable}
	case !unstructured && neutral:
		return origins{neutral: true}
	}
	judged = judged && len(operands) > 0
	if len(rescues) > 0 {
		return origins{nilable: nilable, wraps: []wrap{{call: call, judged: judged, rescues: normalize(rescues)}}}
	}
	return unconditional(call, judged).join(origins{nilable: nilable})
}

// unsettled reports whether an operand can be something other than a judged
// function's error, nil, a cause slot's value or structured: an error made
// here, or one that depends on arguments.
func (o origins) unsettled() bool {
	return o.produced != nil || len(o.deps) > 0 || len(o.wraps) > 0
}

// unconditional is a wrapping no argument rescues.
func unconditional(call *witness, judged bool) origins {
	if judged {
		return origins{viaJudged: true}
	}
	return origins{produced: call}
}
