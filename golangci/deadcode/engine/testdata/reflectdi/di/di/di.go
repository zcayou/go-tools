// Package di is a miniature reflect-typed container: constructors are stored
// as any, invoked through reflect, and products surfaced through an assertion
// on an any-constrained type parameter — so no conversion between a product
// and the interfaces it serves ever appears in a body SSA builds.
package di

import "reflect"

// Applier is the capability the container discovers on a product's metadata.
type Applier[D any] interface {
	Apply(D) error
}

// Provider is the constraint an enrolled provider satisfies; the compiler's
// verification at the enrollment site is the only evidence its methods run.
type Provider[I any] interface {
	Configure(name string)
	ResolveInput(input I) I
}

type Container struct {
	products []any
}

func New() *Container { return &Container{} }

// Register invokes the constructor through reflect and keeps its product.
func (c *Container) Register(ctor any) {
	product := reflect.ValueOf(ctor).Call(nil)[0].Interface()
	c.products = append(c.products, product)
}

// Enroll configures a provider and registers it under its constraint; the
// invocation sits in a body SSA never builds.
func Enroll[I any, P Provider[I]](c *Container, p P) {
	p.Configure("enrolled")
	c.products = append(c.products, p)
}

// Resolve surfaces the first product satisfying R.
func Resolve[R any](c *Container) R {
	for _, product := range c.products {
		if resolved, ok := product.(R); ok {
			return resolved
		}
	}
	var zero R
	return zero
}

// ApplyAll walks each product's Meta field through reflect and applies the
// Applier capability where the metadata carries it.
func ApplyAll[D any](c *Container, defaults D) error {
	for _, product := range c.products {
		value := reflect.ValueOf(product)
		if value.Kind() == reflect.Pointer {
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct {
			continue
		}
		meta := value.FieldByName("Meta")
		if !meta.IsValid() {
			continue
		}
		if applier, ok := meta.Interface().(Applier[D]); ok {
			if err := applier.Apply(defaults); err != nil {
				return err
			}
		}
	}
	return nil
}
