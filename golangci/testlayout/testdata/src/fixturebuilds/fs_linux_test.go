package fixturebuilds_test

import "testing"

func TestLinux(t *testing.T) {
	if tempFS() != "fs" {
		t.Fatal("linux")
	}
}
