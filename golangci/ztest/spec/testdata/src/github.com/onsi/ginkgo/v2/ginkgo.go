// Package ginkgo stands in for Ginkgo, declaring the part the fixtures use.
package ginkgo

import "context"

type SpecContext interface {
	context.Context
}

type TableEntry struct{}

func DescribeTable(description string, args ...any) bool { return true }

func Entry(description any, args ...any) TableEntry { return TableEntry{} }
