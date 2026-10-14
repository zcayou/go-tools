package visibility

import "fmt"

// Each enum below is unexported and declares no name, and module code
// converts a value carrying it to an interface, or does not.

type logged int

const loggedA logged = 1

func logIt() { fmt.Println(loggedA) }

type passed int

const passedA passed = 1

func keep(any) {}

func passIt() { keep(passedA) }

type pointed int

const pointedA pointed = 1

func pointIt() {
	v := pointedA
	fmt.Println(&v)
}

type sliced int

const slicedA sliced = 1

func sliceIt() { fmt.Println([]sliced{slicedA}) }

type arrayed int

const arrayedA arrayed = 1

func arrayIt() { fmt.Println([1]arrayed{arrayedA}) }

type keyed int

const keyedA keyed = 1

type valued int

const valuedA valued = 1

func mapIt() { fmt.Println(map[keyed]valued{keyedA: valuedA}) }

type channeled int

const channeledA channeled = 1

func channelIt() { fmt.Println(make(chan channeled)) }

type wrapped int

const wrappedA wrapped = 1

type secret int

const secretA secret = 1

type wrapper struct {
	Inner  wrapped
	secret secret
}

func wrapIt() { fmt.Println(wrapper{Inner: wrappedA, secret: secretA}) }

type embedded int

const embeddedA embedded = 1

type outer struct {
	embedded
}

func embedIt() { fmt.Println(outer{embeddedA}) }

type initialized int

const initializedA initialized = 1

var rendered = fmt.Sprint(initializedA)

type instantiated int

const instantiatedA instantiated = 1

func show[T any](v T) { fmt.Println(v) }

func instantiateIt() { show(instantiatedA) }

type closed int

const closedA closed = 1

func closeIt() {
	func() { fmt.Println(closedA) }()
}

type compared int

const comparedA compared = 1

func compareIt(c compared) bool { return c == comparedA }
