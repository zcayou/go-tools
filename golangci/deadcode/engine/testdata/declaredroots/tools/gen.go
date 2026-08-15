//go:build ignore

// Command gen is run as go run tools/gen.go.
package main

import (
	"fmt"

	"declaredroots/lib"
)

func main() {
	fmt.Println(lib.Generate())
}

func fatal() {
	panic("gen")
}
