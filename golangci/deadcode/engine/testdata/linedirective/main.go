//line gen.y:1
package main

import "fmt"

// Dead is unreferenced. Its finding has to carry the file the loader opened, not the name
// the directive gives this region, because that name is what a caller looks findings up by.
func Dead() string { return "dead" }

func main() {
	fmt.Println("hi")
}
