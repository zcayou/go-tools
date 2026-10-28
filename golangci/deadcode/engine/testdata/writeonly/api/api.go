// Package api is the module's declared surface.
package api

import "writeonly/plan"

// Handle is what Open hands back, so a consumer may read its exported fields.
type Handle struct {
	Level int
	mode  int
}

func Open() *Handle {
	plan.Prepare()
	return &Handle{Level: 1, mode: 2}
}

// Builder is in no API signature, but a consumer names it and reads what Build
// writes.
type Builder struct {
	Out  string
	step int
}

func (b *Builder) Build() {
	b.Out = "built"
	b.step++
}

func (b *Builder) Result() *plan.Outcome { return &plan.Outcome{Code: 1} }

// Record has no methods, and a consumer names it.
type Record struct{ Note string }

func record() { _ = Record{Note: "n"} }
