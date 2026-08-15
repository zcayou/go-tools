// Package kit is a production-shaped package whose intended consumers are
// tests: nothing but the test file reaches it.
package kit

import "testfacing/lib"

// Greet is the surface the tests exercise.
func Greet() string { return lib.Only() }

// orphan is dead in the full view too: not even tests reach it.
func orphan() string { return "orphan" }
