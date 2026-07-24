package main

import "fmt"

type reporter interface {
	Report() string
}

// Reporter names the same interface through an alias, which since Go 1.23 arrives as its own type
// node. Reading past it is what keeps the laundering rule switched on.
type Reporter = reporter

type Sink struct{}

func (Sink) Report() string { return "sink" }

func hold(r Reporter) Reporter { return r }

func main() {
	fmt.Println(hold(Sink{}))
}
