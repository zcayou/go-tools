package main

import "fmt"

// Holder is boxed, and a generic method's signature is its only mention of Box.
// Reflection cannot call a generic method, so no Box is ever materialized.
type Holder struct{}

func (Holder) Make[F any]() *Box[F] { return nil }

type Box[V any] struct{}

// String has no caller but fmt's Stringer assertion.
func (Box[V]) String() string { return "box" }

// Carrier is boxed, and an unexported method's signature is its only mention
// of Payload. RTA derives Payload through it all the same; reflection cannot.
type Carrier struct{}

func (Carrier) payload() Payload { return Payload{} }

type Payload struct{}

// String has no caller but fmt's Stringer assertion.
func (Payload) String() string { return "payload" }

func main() {
	fmt.Println(Holder{}, Carrier{})
	_ = Holder{}.Make[int]()
	_ = Carrier{}.payload()
}
