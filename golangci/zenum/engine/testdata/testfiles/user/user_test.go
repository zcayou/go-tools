package user_test

import (
	"fmt"
	"testing"

	"example.com/testfiles/kinds"
	"example.com/testfiles/user"
)

type holder struct{ kind kinds.Kind }

func logf(format string, args ...any) { fmt.Printf(format, args...) }

type printing struct{}

func (printing) Logf(format string, args ...any) { fmt.Printf(format, args...) }

func TestUser(t *testing.T) {
	k := user.Default()
	n := 2
	if k.String() == "a" {
	}
	if k == 2 {
	}
	_ = kinds.Kind(n)
	_ = int(k)
	fmt.Printf("%d", k)
	fmt.Println(holder{kind: k})
	_ = k > kinds.KindA
	_ = k <= kinds.KindC
	logf("%d", k)
	_ = user.Convert[kinds.Kind](n)
	var l user.Logger = printing{}
	l.Logf("%d", k)
	user.Report(l, k)
}
