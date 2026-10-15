// Package net takes options from its callers.
package net

import (
	"fmt"
	"time"
)

// Options is what Dial takes.
type Options struct {
	Timeout time.Duration
	Retries int
	Label   string
	Debug   bool
	// Old keeps callers compiling.
	//
	// Deprecated: use Timeout.
	Old     int
	Hint    string
	Tag     string
	Key     string
	Written int
	Nested  Inner
	base
	Limits
}

// Inner is reached through an exported field.
type Inner struct {
	Level  int
	Unused int
}

type base struct{ Shared string }

// Limits is embedded by value, so its fields are promoted onto Options.
type Limits struct{ Max int }

// Result is handed back, so the caller only reads it.
type Result struct{ Count int }

// Config is printed, which reads every field.
type Config struct {
	Name string
	Size int
}

// Plugin is implemented by the caller: Setup is handed an Env, and authors
// a Reply.
type Plugin interface {
	Setup(Env) Reply
}

type Env struct{ Root string }

type Reply struct{ Accepted bool }

// Visitor is handed to the function the caller supplies.
type Visitor struct{ Depth int }

func (o Options) String() string { return o.Label }

// Fingerprint folds a field into an identity.
func (o Options) Fingerprint() string { return o.Key }

func Dial(opts Options) Result {
	tag := func() string { return opts.Tag }
	opts.Written = 1
	shared := opts.base
	return Result{Count: int(opts.Timeout) + len(tag()) + opts.Nested.Level + len(shared.Shared)}
}

func Configure(c Config) { fmt.Println(c) }

func Register(p Plugin) {}

func Visit(fn func(Visitor)) {}

func hint(o Options) string { return o.Hint }
