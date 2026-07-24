// Package pkg is the declared API: nothing in this module consumes it, so what it exempts decides
// what is left to report.
package pkg

// Widget is exported and referenced by nothing here.
type Widget struct{}

// Spin is selected by nobody, and reaches spinHelper alone.
func (Widget) Spin() { spinHelper() }

func spinHelper() {}

// Emit is an exported function nothing calls.
func Emit() {}

// orphanUnexported is unexported, so no consumer can reach it and no exemption covers it.
func orphanUnexported() {}
