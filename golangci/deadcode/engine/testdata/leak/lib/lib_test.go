package lib

import (
	"fmt"
	"testing"
)

var _ = fmt.Sprint(Start, Stop)

func TestLevel(t *testing.T) {
	if fmt.Sprint(High) != "high" {
		t.Fatal("level")
	}
}
