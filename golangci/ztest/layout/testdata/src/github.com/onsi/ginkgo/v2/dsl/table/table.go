// Package table stands in for Ginkgo's dsl/table, which re-exports the root
// package's table builders.
package table

import "github.com/onsi/ginkgo/v2"

var DescribeTable = ginkgo.DescribeTable
