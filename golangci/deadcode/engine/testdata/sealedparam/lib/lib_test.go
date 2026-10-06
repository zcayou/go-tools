package lib_test

import (
	"testing"

	"sealedparam/lib"
)

func TestCompile(t *testing.T) {
	if _, err := lib.Compile[int](lib.NewExplicit[int]()); err != nil {
		t.Fatal(err)
	}
	_ = lib.Describe[int](lib.Label[int]{})
}
