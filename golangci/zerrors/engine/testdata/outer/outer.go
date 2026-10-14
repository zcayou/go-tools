// Package outer uses inner without exposing it.
package outer

import "example.com/inner"

// Run runs.
func Run() int { return inner.Count(nil) }
