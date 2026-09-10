package main

import "testing"

func TestDescribe(t *testing.T) {
	if describe().String() != "label" {
		t.Fatal("wrong label")
	}
}
