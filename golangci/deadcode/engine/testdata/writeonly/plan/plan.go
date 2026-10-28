// Package plan is the module's own, outside the declared surface.
package plan

// Internal is exported, but nothing a consumer holds reaches it.
type Internal struct{ Depth int }

func Prepare() { _ = Internal{Depth: 1} }

// Outcome is what an API method hands back.
type Outcome struct{ Code int }

// Detail is reached only through Outcome's method.
type Detail struct{ Text string }

func (o *Outcome) Detail() *Detail { return &Detail{Text: "t"} }
