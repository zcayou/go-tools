package kinds_test

import (
	"fmt"
	"testing"

	"example.com/testfiles/kinds"
)

type holder struct{ kind kinds.Kind }

func TestOwn(t *testing.T) {
	n := 2
	k := kinds.Kind(n)
	_ = int(k)
	_ = k > kinds.KindA
	fmt.Println(holder{kind: k})
	t.Logf("%x", k)
}
