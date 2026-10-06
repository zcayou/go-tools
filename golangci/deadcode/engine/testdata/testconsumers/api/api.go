// Package api is a declared surface whose consumers are not in the module.
// Its tests stand in for them.
package api

// Registrar is the seam a constructor registers through. Production hands one
// out and never calls it; only a test, as a constructor would, selects Append.
type Registrar interface {
	Append(name string) error
}

// Observer is what Notify reports to.
type Observer interface {
	Observe(event string)
}

// Notify reports event to observer.
func Notify(observer Observer, event string) { observer.Observe(event) }

// ObserverFunc adapts a func to Observer, and only the test uses it.
type ObserverFunc func(event string)

func (f ObserverFunc) Observe(event string) { f(event) }

// Meter is held only by the test, which reads it through an interface
// of its own, the way a consumer over a public type would.
type Meter struct{}

func (Meter) Read() int { return meterHelper() }

func meterHelper() int { return 1 }

// HandlerFunc and ListenerFunc are twins nothing references, tests included.
// facade aliases HandlerFunc, which must not keep its method alive.
type HandlerFunc func()

func (f HandlerFunc) Handle() { handled() }

type ListenerFunc func()

func (f ListenerFunc) Listen() { listened() }

func handled() {}

func listened() {}
