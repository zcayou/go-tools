// Command lazybodies converts a tracker to a Publisher only inside a method
// x/tools builds once two rounds of methods of runtime types have been built:
// main boxes a session[string], whose Run boxes an attempt[string], whose Go
// boxes the tracker.
package main

// Publisher is selected below, so a bind to it confers a use.
type Publisher[R any] interface{ Publish(value R) }

type tracker[R any] struct{ last R }

func (t *tracker[R]) Publish(value R) { t.last = value }

func emit[R any](publisher Publisher[R]) {
	var zero R
	publisher.Publish(zero)
}

type goer interface{ Go() }

type attempt[R any] struct{}

func (attempt[R]) Go() { emit[R](&tracker[R]{}) }

type runner interface{ Run() }

type session[R any] struct{}

func (session[R]) Run() {
	var g goer = attempt[R]{}
	g.Go()
}

func main() {
	var r runner = session[string]{}
	r.Run()
}
