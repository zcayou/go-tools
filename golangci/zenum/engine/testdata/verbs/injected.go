package verbs

import (
	"fmt"
	"log"
)

// Options takes its loggers as funcs, as a library does that keeps its callers
// free of a logging dependency.
type Options struct {
	Trace  func(format string, args ...any)
	Report func(format string, args ...any)
	Quiet  func(format string, args ...any)
	Mixed  func(format string, args ...any)
	Logf   func(format string, args ...any)
	Emit   func(msg string, args ...any)
}

type client struct {
	opts  Options
	trace func(format string, args ...any)
}

var debug = func(format string, args ...any) { fmt.Printf(format, args...) }

func newClient(opts Options) *client {
	opts.Trace = func(format string, args ...any) { log.Printf(format, args...) }
	opts.Report = log.Printf
	opts.Quiet = func(string, ...any) {}
	opts.Quiet = debug
	opts.Mixed = func(msg string, args ...any) { fmt.Println(args...) }
	opts.Mixed = func(format string, args ...any) { fmt.Printf(format, args...) }
	return &client{opts: opts, trace: opts.Trace}
}

// Journal has a no-op implementation beside a printing one.
type Journal interface{ Logf(format string, args ...any) }

type quietJournal struct{}

func (quietJournal) Logf(string, ...any) {}

type stdJournal struct{}

func (stdJournal) Logf(format string, args ...any) { fmt.Printf(format, args...) }

func journal(quiet bool) Journal {
	if quiet {
		return quietJournal{}
	}
	return stdJournal{}
}

// Sink's only relation to fileSink is a type assertion.
type Sink interface{ Logf(format string, args ...any) }

type fileSink struct{}

func (fileSink) Logf(format string, args ...any) { fmt.Printf(format, args...) }

func flush(s Sink) bool {
	_, ok := s.(fileSink)
	return ok
}

// Logf is a logger type a caller passes in.
type Logf func(format string, args ...any)

func injected(k Kind, c *client, s Sink, out Logf, emit func(string, ...any)) {
	c.opts.Trace("%d", k)
	c.opts.Report("%d", k)
	c.opts.Quiet("%d", k)
	c.opts.Mixed("%d", k)
	c.opts.Logf("%d", k)
	c.trace("%d", k)
	debug("%d", k)
	journal(false).Logf("%d", k)
	s.Logf("%d", k)
	out("%d", k)
	emit("%d", k)
	c.opts.Emit("%d", k)
}
