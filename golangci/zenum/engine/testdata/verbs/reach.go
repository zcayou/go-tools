package verbs

import "fmt"

type pair struct {
	Exported Kind
	hidden   Kind
}

type job struct {
	name  string
	state Kind
}

type Wrapper struct {
	inner job
}

type named struct{ K Kind }

func (named) String() string { return "named" }

type formatted struct{ K Kind }

func (formatted) Format(fmt.State, rune) {}

type Flag uint8

const (
	FlagA Flag = iota + 1
	FlagB
)

func (f Flag) String() string {
	switch f {
	case FlagA:
		return "read"
	case FlagB:
		return "write"
	default:
		return "flag"
	}
}

func recursion(ks []Kind, p pair, m map[Kind]int, ptr *pair, pk *Kind, n named, f formatted) {
	fmt.Printf("%d\n", ks)
	fmt.Printf("%d\n", p)
	fmt.Printf("%d\n", m)
	fmt.Printf("%d\n", ptr)
	fmt.Printf("%d\n", pk)
	fmt.Printf("%d\n", n)
	fmt.Printf("%x\n", n)
	fmt.Printf("%d\n", f)
}

func bypass(p pair, j job, w Wrapper, n named, flags []Flag, ks []Kind) {
	fmt.Printf("%v\n", p)
	fmt.Printf("%+v\n", j)
	fmt.Println(w)
	fmt.Printf("%q\n", flags)
	fmt.Printf("%v\n", n)
	fmt.Printf("%v\n", flags)
	fmt.Printf("%#v\n", j)
	fmt.Printf("%v\n", ks)
}
