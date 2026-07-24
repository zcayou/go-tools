package main

import (
	"fmt"

	"plain/sub"
)

// Dead is what the pass over this package has to report, on this line.
func Dead() string { return "dead" }

func main() {
	fmt.Println(sub.Used())
}
