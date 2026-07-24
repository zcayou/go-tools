// Package orphan is imported by nothing, so a scan restricted to the roots' import closure would
// report none of it.
package orphan

func stranded() string { return "stranded" }
