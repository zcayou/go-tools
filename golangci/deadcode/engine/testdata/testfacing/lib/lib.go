// Package lib is production code main keeps alive.
package lib

// Kept is what main consumes.
func Kept() string { return "kept" }

// Only is consumed by nothing but the kit package.
func Only() string { return "only" }
