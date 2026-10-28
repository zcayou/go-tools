package plan_test

import (
	"testing"

	"writeonly/kit"
)

func TestFixture(t *testing.T) {
	if kit.NewFixture().X != 1 {
		t.Fatal("X")
	}
}
