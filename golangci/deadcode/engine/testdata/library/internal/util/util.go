// Package util is internal, so no consumer can reach it and it contributes no roots of its own.
package util

// Prefix is reached from the library's public surface.
func Prefix() string { return "> " }

// Dangling is exported but internal, and nothing in the module calls it.
func Dangling() string { return "dangling" }
