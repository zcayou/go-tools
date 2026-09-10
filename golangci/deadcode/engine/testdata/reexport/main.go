package main

import (
	"fmt"

	"reexport/facade"
	"reexport/outer"
)

func main() {
	var mode facade.Mode = outer.On
	fmt.Println(mode)
}
