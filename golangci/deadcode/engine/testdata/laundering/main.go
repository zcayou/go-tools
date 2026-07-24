package main

import "fmt"

// Reporter is declared here and nothing selects Report, so binding Sink to it must not launder
// Sink.Report into looking used.
type Reporter interface {
	Report() string
}

type Sink struct{}

func (Sink) Report() string { return "sink" }

// hold converts a Sink to a Reporter, which is the bind the rule has to refuse rather than
// never see.
func hold(r Reporter) Reporter { return r }

func main() {
	fmt.Println(hold(Sink{}))
}
