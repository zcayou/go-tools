package main

import "testing"

// TestTested is the only reference to Tested anywhere in the module.
func TestTested(t *testing.T) {
	if Tested() != "tested" {
		t.Fatal("wrong value")
	}
}
