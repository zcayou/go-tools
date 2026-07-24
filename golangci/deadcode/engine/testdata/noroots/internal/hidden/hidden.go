// Package hidden is internal, so it contributes no roots, and analyzing it alone leaves
// reachability undefined rather than empty.
package hidden

func hidden() {}
