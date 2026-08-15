package main

import (
	"testing"

	"testfacing/kit"
)

// TestGreet is the only consumer of kit.
func TestGreet(t *testing.T) {
	if kit.Greet() != "only" {
		t.Fatal("wrong greeting")
	}
}
