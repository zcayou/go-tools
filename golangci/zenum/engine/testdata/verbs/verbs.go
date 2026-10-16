package verbs

import (
	"fmt"
	"log"
)

type Kind int

const (
	KindA Kind = iota + 1
	KindB
)

func (k Kind) String() string {
	switch k {
	case KindA:
		return "a"
	case KindB:
		return "b"
	default:
		return fmt.Sprintf("Kind(%d)", k)
	}
}

type Mode string

const ModeRead Mode = "read"

func bypassing(k Kind) {
	fmt.Printf("%d %x %X %o %b %c %U\n", k, k, k, k, k, k, k)
}

func naming(k Kind, m Mode, n int) {
	fmt.Printf("%v %s %+v %q\n", k, k, k, k)
	fmt.Printf("%d %x\n", n, m)
}

func indexed(k Kind, logger *log.Logger) error {
	_ = fmt.Sprintf("%[2]d %[1]s", k, k)
	log.Printf("%*d %%d %s", 3, k, k)
	logger.Printf("%-8.*x", 2, k)
	args := []any{k}
	fmt.Printf("%d", args...)
	return fmt.Errorf("kind %d", k)
}

func malformed(k Kind, n int) {
	fmt.Printf("%[0]d %[]d %[x]d %[99999999999]d %[3]d %d", k, n)
	fmt.Printf("%[2]d", k)
}

func octal(k Kind, n named) {
	fmt.Printf("%O %#O\n", k, k)
	fmt.Printf("%O\n", n)
}
