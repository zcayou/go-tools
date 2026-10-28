package wholemodule

import (
	"go/types"
	"strconv"

	"golang.org/x/tools/go/ssa"
)

// Sink is what a standard-library function or method that takes a value
// as an interface, and never renders it, does with that value.
type Sink int

const (
	// SinkHolds keeps the value, sorts it, or describes its type, and reads
	// nothing of what it holds.
	SinkHolds Sink = iota + 1

	// SinkCompares compares the value with another, as a key or as the old
	// value of a compare-and-swap.
	SinkCompares
)

func (s Sink) String() string {
	switch s {
	case SinkHolds:
		return "holds"
	case SinkCompares:
		return "compares"
	default:
		return "Sink(" + strconv.Itoa(int(s)) + ")"
	}
}

// sinks are the standard-library sinks, by [types.Func.FullName], with what
// each does with the argument at each index of [ssa.CallCommon.Args]. A method
// called statically has its receiver at index zero; one invoked through
// an interface does not. An index the slice leaves out, or holds zero for,
// is no sink's.
var sinks = map[string][]Sink{
	"context.WithValue":                   {0, SinkCompares, SinkHolds},
	"(context.Context).Value":             {SinkCompares},
	"(*sync.Map).CompareAndDelete":        {0, SinkCompares, SinkCompares},
	"(*sync.Map).CompareAndSwap":          {0, SinkCompares, SinkCompares, SinkHolds},
	"(*sync.Map).Delete":                  {0, SinkCompares},
	"(*sync.Map).Load":                    {0, SinkCompares},
	"(*sync.Map).LoadAndDelete":           {0, SinkCompares},
	"(*sync.Map).LoadOrStore":             {0, SinkCompares, SinkHolds},
	"(*sync.Map).Store":                   {0, SinkCompares, SinkHolds},
	"(*sync.Map).Swap":                    {0, SinkCompares, SinkHolds},
	"(*sync/atomic.Value).CompareAndSwap": {0, SinkCompares, SinkHolds},
	"(*sync/atomic.Value).Store":          {0, SinkHolds},
	"(*sync/atomic.Value).Swap":           {0, SinkHolds},
	"sort.Slice":                          {SinkHolds},
	"sort.SliceIsSorted":                  {SinkHolds},
	"sort.SliceStable":                    {SinkHolds},
	"reflect.TypeOf":                      {SinkHolds},
}

// SinkOf returns what the standard-library sink call names does with v,
// an argument of the call: [SinkCompares] when any argument v is compares it,
// [SinkHolds] when every one holds it, and zero when the callee is no sink,
// v is no argument of the call, or the callee may render v at one of them.
func SinkOf(call *ssa.CallCommon, v ssa.Value) Sink {
	uses := sinks[calleeName(call)]
	var found Sink
	for i, arg := range call.Args {
		if arg != v {
			continue
		}
		if i >= len(uses) || uses[i] == 0 {
			return 0
		}
		found = max(found, uses[i])
	}
	return found
}

// calleeName is the full name of the function or interface method a call
// statically names, or "".
func calleeName(call *ssa.CallCommon) string {
	if call.IsInvoke() {
		return call.Method.FullName()
	}
	if callee := call.StaticCallee(); callee != nil {
		if fn, ok := callee.Object().(*types.Func); ok {
			return fn.FullName()
		}
	}
	return ""
}
