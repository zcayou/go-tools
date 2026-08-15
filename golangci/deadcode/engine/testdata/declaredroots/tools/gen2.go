//go:build ignore

// Command gen2 collides with gen: same directory, same main, same fatal.
package main

import (
	"fmt"

	"declaredroots/lib"
)

func main() {
	fmt.Println(lib.Sweep())
}

func fatal() {
	panic("gen2")
}
