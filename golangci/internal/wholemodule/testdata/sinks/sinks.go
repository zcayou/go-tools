package sinks

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
)

type Value struct{ N int }

func ContextKey(ctx context.Context, v Value) context.Context { return context.WithValue(ctx, v, 1) }

func ContextValue(ctx context.Context, v Value) context.Context {
	return context.WithValue(ctx, "k", v)
}

func ContextLookup(ctx context.Context, v Value) any { return ctx.Value(v) }

func MapKey(m *sync.Map, v Value) { m.Store(v, 1) }

func MapValue(m *sync.Map, v Value) { m.Store(1, v) }

func MapBoth(m *sync.Map, v Value) { m.Store(v, v) }

func MapOld(m *sync.Map, v Value) bool { return m.CompareAndSwap(1, v, 2) }

func AtomicStore(a *atomic.Value, v Value) { a.Store(v) }

func AtomicOld(a *atomic.Value, v Value) bool { return a.CompareAndSwap(v, 2) }

func Sorted(s []Value) { sort.Slice(s, func(i, j int) bool { return s[i].N < s[j].N }) }

func TypeOf(v Value) reflect.Type { return reflect.TypeOf(v) }

func Printed(v Value) { show(v) }

func show(x any) { fmt.Println(x) }
