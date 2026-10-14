package lib

import "fmt"

func Hello() string {
	return fmt.Sprint("a", 1)
}

func Spread(xs []any) string {
	return fmt.Sprint(xs...)
}

func None() string {
	return fmt.Sprint()
}
