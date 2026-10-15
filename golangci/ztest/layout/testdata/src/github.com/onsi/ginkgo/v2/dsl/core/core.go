// Package core stands in for Ginkgo's dsl/core, which re-exports the root
// package's builders for a file that dot-imports only those.
package core

import "github.com/onsi/ginkgo/v2"

var Context = ginkgo.Context
