package main

// Speaker is converted to and selected through only in the test file.
type Speaker interface {
	Speak() string
}

// Dog is constructed by main, so the type itself is live in both views.
type Dog struct{}

func (Dog) Speak() string { return "woof" }

// Bark is selected by nothing anywhere, so its verdicts stay plain.
func (Dog) Bark() string { return "bark" }

// Threshold, Registry, and Mode are referenced only from the test file.
const Threshold = 3

var Registry = map[string]int{}

type Mode int

func main() {
	_ = Dog{}
}
