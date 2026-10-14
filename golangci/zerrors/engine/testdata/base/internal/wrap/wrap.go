// Package wrap is internal, and its errors leave the module wrapped in API ones.
package wrap

type Exec struct{}

func (*Exec) Error() string { return "exec" }
