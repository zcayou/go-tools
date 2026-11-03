// Package structconstraint builds structs through type parameters whose
// constraint's core type is a struct, which is the one way Go lets code reach
// a type parameter's fields.
package structconstraint

import "errors"

type registry struct{ name string }

type holder struct{ registry *registry }

type shape interface{ ~struct{ registry *registry } }

func positional[F interface{ ~struct{ registry *registry } }]() F {
	return F{&registry{name: "positional"}}
}

func keyed[F shape]() F {
	return F{registry: &registry{name: "keyed"}}
}

type mirror struct{ registry *registry }

func either[F holder | mirror]() F {
	return F{&registry{name: "either"}}
}

// Result carries the error Run returns.
type Result struct{ Err error }

func fill[R interface{ ~struct{ Err error } }]() R {
	return R{Err: errors.New("raw")}
}

func Run() error {
	_, _, _ = positional[holder](), keyed[holder](), either[mirror]()
	return fill[Result]().Err
}
