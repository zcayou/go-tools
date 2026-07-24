// Package sub holds no findings of its own, so a pass over it alone has to drop the module's
// findings rather than place them somewhere it can.
package sub

// Used is called from the main package.
func Used() string { return "used" }
