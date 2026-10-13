package fixturebuilds_test

import "testing"

func TestWindows(t *testing.T) {
	if tempFS() != "fs" {
		t.Fatal("windows")
	}
}
