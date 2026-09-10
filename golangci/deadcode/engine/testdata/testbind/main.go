package main

import "fmt"

// Label reaches an interface only through describe, which nothing but the test
// calls. The bind to fmt.Stringer credits String, so its own verdict is spared
// in either view; but the conversion is one the program makes, and where
// nothing runs it, nothing runs String or what String calls.
type Label struct{ text string }

func (l Label) String() string { return render(l.text) }

func render(text string) string { return text }

func describe() fmt.Stringer { return Label{text: "label"} }

func main() {}
