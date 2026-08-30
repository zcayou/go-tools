package main

// Live is declared and selected in production, so the production side is the
// fixture's control: none of it is reported.
type Live interface {
	Emit() string
}

type Source struct{}

func (Source) Emit() string { return "source" }

func main() {
	var live Live = Source{}
	println(live.Emit())
}
