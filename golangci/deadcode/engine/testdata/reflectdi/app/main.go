// The application wires everything through the reflect-typed container, so
// no conversion from a concrete component to the interfaces serving it exists
// in any body SSA builds.
package main

import (
	"fmt"

	"difixture/di"

	"app/component"
	"app/facade"
	"app/meta"
	"app/provider"
)

type config struct{ name string }

func main() {
	c := di.New()
	c.Register(facade.NewRuntime(config{name: "fixture"}))
	c.Register(component.NewCache)
	c.Register(meta.NewSpec)
	c.Register(meta.NewMismatch)
	di.Enroll[provider.Input, provider.Impl](c, provider.Impl{})

	if err := di.ApplyAll(c, meta.Defaults{Limit: 3}); err != nil {
		panic(err)
	}

	authority := di.Resolve[facade.Authority](c)
	authority.Push("item")
	fmt.Println(authority.Pull())

	store := di.Resolve[component.Store](c)
	store.Save("state")
}
