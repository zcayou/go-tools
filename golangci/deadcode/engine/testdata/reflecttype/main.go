// The program's only materialization of hidden is the descriptor
// reflect.TypeFor conjures, reached through a generic helper so the argument
// resolves transitively.
package main

import (
	"fmt"
	"reflect"
)

type greeter interface {
	greet() string
}

type hidden struct{}

func (hidden) greet() string { return "hidden" }

// shadow is shaped exactly like hidden, and no reflect.TypeFor names it.
type shadow struct{}

func (shadow) greet() string { return "shadow" }

func descriptor[T any]() reflect.Type {
	return reflect.TypeFor[T]()
}

func main() {
	if g, ok := reflect.Zero(descriptor[hidden]()).Interface().(greeter); ok {
		fmt.Println(g.greet())
	}
}
