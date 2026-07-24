package main

import "fmt"

// ExampleCompute carries no output comment, so cmd/go compiles it and never registers it: nothing
// reaches it, or the helper only it calls, through the synthesized test main.
func ExampleCompute() {
	fmt.Println(double(Compute()))
}

func double(n int) int { return n * 2 }
