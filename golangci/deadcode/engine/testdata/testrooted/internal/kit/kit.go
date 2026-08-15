// Package kit has no production entry points: internal packages contribute
// no fallback roots, so only the test binary roots this module.
package kit

func Helper() int { return 1 }
