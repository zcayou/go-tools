package kinds

import (
	"fmt"
	"testing"
)

func TestInside(t *testing.T) {
	n := 2
	k := Kind(n)
	_ = int(k)
	_ = k > KindA
	_ = k <= KindC
	t.Logf("%d", k)
	_ = fmt.Sprint(secretA, secretB)
}
