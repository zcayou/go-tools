package lib

import "testing"

// TestGenerate makes Generate a declaration a test and a generator both
// reach.
func TestGenerate(t *testing.T) {
	if Generate() != "generated" {
		t.Fatal("wrong output")
	}
}
