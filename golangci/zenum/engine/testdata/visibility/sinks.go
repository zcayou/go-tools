package visibility

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
)

// Each enum below is handed to an interface only by a standard library
// sink that never renders it, except escaped, which fmt is handed too.

type ctxKey int

const ctxKeyUser ctxKey = 1

func withUser(ctx context.Context) context.Context { return context.WithValue(ctx, ctxKeyUser, "user") }

func user(ctx context.Context) any { return ctx.Value(ctxKeyUser) }

type sorted int

const sortedA sorted = 1

func sortIt() {
	xs := []sorted{sortedA}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
}

type stored int

const storedA stored = 1

var syncMap sync.Map

func storeIt() {
	syncMap.Store(storedA, storedA)
	syncMap.Load(storedA)
}

type atomized int

const atomizedA atomized = 1

var atomicValue atomic.Value

func atomizeIt() { atomicValue.Store(atomizedA) }

type reflected int

const reflectedA reflected = 1

func reflectIt() reflect.Type { return reflect.TypeOf(reflectedA) }

type escaped int

const escapedA escaped = 1

func escapeIt() {
	v := any(escapedA)
	_ = reflect.TypeOf(v)
	fmt.Println(v)
}
