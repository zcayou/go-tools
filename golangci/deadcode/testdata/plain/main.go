package main

import (
	"fmt"

	"plain/sub"
)

// Dead is what the pass over this package has to report, on this line.
func Dead() string { return "dead" }

// Tested is referenced only from the test file, the shape the test-only family
// reports.
func Tested() string { return "tested" }

func main() {
	fmt.Println(sub.Used())
}
