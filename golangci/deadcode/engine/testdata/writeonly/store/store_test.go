package store

import "testing"

func TestGauge(t *testing.T) {
	var g Gauge
	g.Set(3)
	if g.level != 3 {
		t.Fatal("level")
	}
}

// fake is declared by the test.
type fake struct{ calls int }

func TestFake(t *testing.T) {
	f := &fake{}
	f.calls++
}
