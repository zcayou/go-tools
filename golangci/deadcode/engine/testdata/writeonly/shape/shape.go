// Package shape declares structs that share one shape, each read its own way.
package shape

import (
	"fmt"
	"reflect"
)

func show(v any) string { return fmt.Sprint(v) }

type Spec[T any] struct{ Default T }

func Register[T any](s Spec[T]) string { return show(s.Default) }

func Walk[T any](s Spec[T]) int { return reflect.ValueOf(s.Default).NumField() }

func Describe[T any]() reflect.Type { return reflect.TypeFor[T]() }

type boxedInGeneric struct{ Value string }

type walkedInGeneric struct{ Value string }

type boxedDirectly struct{ Value string }

type describedInGeneric struct{ Value string }

type describedDirectly struct{ Value string }

func Use() (string, string, int) {
	a := describedInGeneric{Value: "x"}
	b := describedDirectly{Value: "x"}
	_, _ = a, b
	_ = Describe[describedInGeneric]()
	_ = reflect.TypeFor[describedDirectly]()
	return Register(Spec[boxedInGeneric]{Default: boxedInGeneric{Value: "x"}}),
		show(boxedDirectly{Value: "x"}),
		Walk(Spec[walkedInGeneric]{Default: walkedInGeneric{Value: "x"}})
}

type element struct {
	Value  string
	hidden string
}

type scalar struct {
	Value  string
	hidden string
}

func Same() bool {
	return element{Value: "a", hidden: "b"} == element{} && scalar{Value: "a", hidden: "b"} == scalar{}
}
