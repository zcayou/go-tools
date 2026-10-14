// Package tools is tooling that stays outside the framework on purpose.
package tools

type Failed struct{}

func (*Failed) Error() string { return "failed" }
