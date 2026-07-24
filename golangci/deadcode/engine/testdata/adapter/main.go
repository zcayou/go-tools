// Every selection happens through interfaces main re-declares identically,
// so the declarations in profiles are load-bearing only through
// interface-to-interface flows.
package main

import (
	"fmt"

	"adapter/profiles"
)

// porter re-declares Profiles as a plain interface; passing a Profiles value
// into it converts between identical method sets.
type porter interface {
	List() []string
}

func port(p porter) []string { return p.List() }

// namer re-declares Registry with the element type abstracted, the way
// a generic consumer re-declares the interface it takes.
type namer[D any] interface {
	Names() []D
}

func names[D any, R namer[D]](r R) []D { return r.Names() }

// evaluator re-declares Evaluator; the two convert into each other inside
// opposite-direction closures.
type evaluator interface {
	Snapshot() map[string]string
}

func bridge() (func(profiles.Evaluator) evaluator, func(evaluator) profiles.Evaluator) {
	return func(e profiles.Evaluator) evaluator { return e },
		func(e evaluator) profiles.Evaluator { return e }
}

func main() {
	fmt.Println(port(profiles.New()))
	fmt.Println(names[string, profiles.Registry](profiles.NewRegistry()))

	toLocal, toShared := bridge()
	local := toLocal(profiles.NewEvaluator())
	fmt.Println(local.Snapshot())
	fmt.Println(toShared(local) != nil)

	// Ignored flows nowhere; Watched reaches porter only through an assertion.
	var ignored profiles.Ignored
	_ = ignored
	var watched profiles.Watched
	if p, ok := watched.(porter); ok {
		fmt.Println(p.List())
	}
}
