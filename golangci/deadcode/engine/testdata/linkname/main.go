package main

import (
	"fmt"
	_ "unsafe"
)

//go:linkname Pushed runtime.deadcodeFixtureHook

// Pushed runs under another package's symbol, so no reference to it appears anywhere. The blank
// line above detaches the directive from this doc comment without disabling it.
func Pushed() string { return helper() }

func helper() string { return "helper" }

func main() {
	fmt.Println("hi")
}
