// Package ginkgo stands in for Ginkgo, declaring the part the fixtures use.
package ginkgo

import "context"

type SpecContext interface {
	context.Context
}
