package main

import "fmt"

// Live is called from main, so nothing reports it.
func Live() string { return "live" }

// Dead is exported and nothing calls it, which draws both verdicts at once: no root reaches it,
// and no reference names it.
func Dead() string { return helper() }

// helper is reached only from Dead, so it dies with it.
func helper() string { return "helper" }

func main() {
	fmt.Println(Live())
}
