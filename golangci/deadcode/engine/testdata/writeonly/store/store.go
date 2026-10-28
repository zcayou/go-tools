// Package store holds fields its code writes and nothing reads.
package store

import (
	"context"
	"sync/atomic"
)

// Literal is only ever built.
type Literal struct {
	Name string
	size int
}

// Pair is built positionally.
type Pair struct{ left, right int }

func Build() {
	_ = Literal{Name: "n", size: 1}
	_ = Pair{1, 2}
}

// Counter is written in every way and read in none.
type Counter struct {
	assigned int
	sum      int
	hits     int
	log      []string
	total    atomic.Int64
	legacy   int64
}

func (c *Counter) Record(entry string, n int) {
	c.assigned = n
	c.sum += n
	c.hits++
	c.log = append(c.log, entry)
	c.total.Add(1)
	atomic.AddInt64(&c.legacy, 1)
}

// Nest is written beneath a struct-valued field.
type Nest struct{ inner part }

type part struct{ v int }

func (n *Nest) Set() { n.inner.v = 1 }

// Box is generic, and its instantiations share its fields.
type Box[T any] struct {
	value T
	stamp int
}

func (b *Box[T]) Put(v T) {
	b.value = v
	b.stamp++
}

func (b *Box[T]) Get() T { return b.value }

func Boxes() (int, string) {
	var i Box[int]
	i.Put(1)
	var s Box[string]
	s.Put("s")
	return i.Get(), s.Get()
}

func Local() {
	type local struct{ n int }
	l := local{n: 1}
	_ = l
}

// Gauge is read only by its test.
type Gauge struct{ level int }

func (g *Gauge) Set(n int) { g.level = n }

// Token is held by a context, which reads nothing of it; ctxKey is compared
// as its key.
type Token struct{ id string }

type ctxKey struct{ name string }

func WithToken(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{name: "token"}, Token{id: "t"})
}

// Snapshot is held by an atomic pointer, whose body is not the module's.
type Snapshot struct{ rev int }

var current atomic.Pointer[Snapshot]

func Publish(rev int) { current.Store(&Snapshot{rev: rev}) }

// Probe is read by kit.
type Probe struct{ Depth int }

func (p *Probe) Sink(n int) { p.Depth = n }

var settings struct{ retries int }

func Configure() { settings.retries = 3 }
