// Package lib is reached almost entirely by the ignore-tagged generators.
package lib

// Generate is what tools/gen.go runs.
func Generate() string { return helper() }

// Sweep is what tools/gen2.go runs.
func Sweep() string { return "swept" }

// Untouched is reached by no generator, so its findings survive.
func Untouched() string { return "untouched" }

func helper() string { return "generated" }
