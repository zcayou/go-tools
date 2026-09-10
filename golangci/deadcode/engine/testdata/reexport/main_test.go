package main

import (
	"testing"

	"reexport/facade"
)

func TestOff(t *testing.T) {
	var box facade.Box[facade.Mode]
	if box.Value == facade.Off {
		t.Fatal("the zero value is a mode")
	}
}
