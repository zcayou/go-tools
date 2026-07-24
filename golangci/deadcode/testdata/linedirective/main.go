//line gen.y:1
package main

import "fmt"

// Dead is unreferenced. The directive above renames this region to a file no pass holds, so
// indexing findings by the adjusted name would lose them silently.
func Dead() string { return "dead" }

func main() {
	fmt.Println("hi")
}
