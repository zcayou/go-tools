package main

import "fmt"

type Gauge struct{}

// Read is exported, bound to nothing, and selected by nobody.
func (Gauge) Read() int { return 1 }

func main() {
	fmt.Println(Gauge{})
}
