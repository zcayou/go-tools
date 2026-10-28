package store

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"sync/atomic"
	"time"
	"unsafe"
)

// Conf's fields are each read one way.
type Conf struct {
	name  string
	port  int
	timer *time.Timer
	index map[string]int
	dead  int
}

func NewConf() *Conf {
	c := &Conf{name: "c", port: 80, timer: time.NewTimer(time.Second), index: map[string]int{}, dead: 1}
	c.index["a"] = 1
	c.timer.Stop()
	bump(&c.port)
	return c
}

func (c *Conf) Name() string { return c.name }

func bump(p *int) { *p++ }

func unusedReader(c *Conf) int { return c.dead }

// Version is read by its String.
type Version struct{ major int }

func (v Version) String() string { return strconv.Itoa(v.major) }

func Current() string { return Version{major: 1}.String() }

// Report is printed, which reads what lies beneath its unexported field.
type Report struct {
	title string
	meta  meta
}

type meta struct{ author string }

func Print() { fmt.Printf("%v\n", Report{title: "t", meta: meta{author: "a"}}) }

// Schema is described by reflection.
type Schema struct{ version int }

func Describe() reflect.Type {
	_ = Schema{version: 1}
	return reflect.TypeFor[Schema]()
}

// State is compared deeply.
type State struct{ phase int }

func Same(a State) bool { return reflect.DeepEqual(a, State{phase: 1}) }

// Point is compared with ==.
type Point struct{ x, y int }

func AtOrigin(p Point) bool { return p == Point{x: 0, y: 0} }

// Key keys a map.
type Key struct{ shard int }

func Index() map[Key]int { return map[Key]int{{shard: 1}: 2} }

// Tag is searched for.
type Tag struct{ label string }

func Tagged(tags []Tag) bool { return slices.Contains(tags, Tag{label: "a"}) }

// Mode is switched on.
type Mode struct{ code int }

func Off(m Mode) bool {
	switch m {
	case Mode{code: 0}:
		return true
	}
	return false
}

// record is converted to another struct type.
type record struct{ id int }

type wire struct{ id int }

func Encode() int { return wire(record{id: 1}).id }

// header is converted to an unsafe.Pointer.
type header struct{ n int }

func Raw() unsafe.Pointer { return unsafe.Pointer(&header{n: 1}) }

// Flags is read by Load and CompareAndSwap.
type Flags struct {
	ready atomic.Bool
	state atomic.Int32
}

func (f *Flags) Toggle() bool {
	f.ready.Store(true)
	f.state.Store(1)
	return f.ready.Load() && f.state.CompareAndSwap(1, 2)
}

// Outer embeds Inner, which no one owns as a field.
type Outer struct{ Inner }

type Inner struct{ depth int }

func (o *Outer) Reset() int {
	o.Inner = Inner{depth: 2}
	return o.depth
}
