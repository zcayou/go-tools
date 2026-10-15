package unparsable

// This variant does not parse, and no build the fixture runs under compiles
// it, so what it declares is unknown to the linter alone.
func (r *Report) Error() string {
