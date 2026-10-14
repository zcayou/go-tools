// Package codes is imported by the base's own package, so its error types
// cannot embed the base.
package codes

type Code int

type Invalid struct{}

func (*Invalid) Error() string { return "invalid" }
