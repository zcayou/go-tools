package main

import "fmt"

// Render constrains on a dependency's interface but never calls its method, so
// the constraint is the only evidence Widget.String participates.
func Render[T fmt.Stringer](v T) string { return fmt.Sprintf("%T", v) }

type Widget struct{}

func (Widget) String() string { return "widget" }

// Feeder's method signature mentions the type parameter, so only its
// substituted form can match a concrete implementation.
type Feeder[D any] interface {
	Feed(D) D
}

func Consume[D any, F Feeder[D]](f F, d D) D { return f.Feed(d) }

type Doubler struct{}

func (Doubler) Feed(n int) int { return n * 2 }

// Keyed mixes a type set with a parameter-free method; the union never
// reaches substitution, because only method signatures are substituted.
type Keyed interface {
	~int | ~string
	Key() string
}

func Index[K Keyed](k K) string { return k.Key() }

type Token int

func (Token) Key() string { return "token" }

func main() {
	fmt.Println(Render(Widget{}))
	fmt.Println(Consume[int, Doubler](Doubler{}, 21))
	fmt.Println(Index(Token(1)))
}
