// Package jobs publishes events to the observers its callers install, and
// takes the rest of what it reads from them.
package jobs

import "context"

// Event is what an observer is handed.
type Event struct {
	Job     string
	Attempt int
}

// Observer is how a caller watches jobs run.
type Observer interface{ Observe(Event) }

// ObserverFunc adapts a func to Observer.
type ObserverFunc func(Event)

func (f ObserverFunc) Observe(e Event) { f(e) }

// Tee returns an Observer handing each event to every observer given.
func Tee(observers ...Observer) Observer { return tee(observers) }

type tee []Observer

func (t tee) Observe(e Event) {
	for _, o := range t {
		o.Observe(e)
	}
}

// Recorder keeps the events it observes.
type Recorder struct{ events []Event }

func (r *Recorder) Observe(e Event) { r.events = append(r.events, e) }

// Events returns what the recorder kept.
func (r *Recorder) Events() []Event { return r.events }

// RetryInfo is handed to the func a caller installs.
type RetryInfo struct{ Wait int }

// Options is what Run takes.
type Options struct {
	Observer Observer
	OnRetry  func(RetryInfo)
	Retries  int
}

// Replay hands a caller's retry back to it.
func Replay(RetryInfo) {}

func Run(ctx context.Context, opts Options) error {
	opts.Observer.Observe(Event{Job: "build", Attempt: 1})
	opts.OnRetry(RetryInfo{Wait: 2})
	return ctx.Err()
}
