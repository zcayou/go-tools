// Package library has no main package, so its exported surface stands in as the roots.
package library

import "library/internal/util"

// Greet is the public entry point.
func Greet() string { return util.Prefix() + "hello" }

// Unused is exported and referenced by nothing in this module, which the exported-surface fallback
// reports all the same.
func Unused() string { return stranded() }

func stranded() string { return "stranded" }
