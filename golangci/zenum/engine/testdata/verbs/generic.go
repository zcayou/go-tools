package verbs

import "fmt"

func show[T any](v T) { fmt.Printf("%d\n", v) }

func shown() {
	show(KindA)
	show(3)
}
