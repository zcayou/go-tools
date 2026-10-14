package zerrors

// LoadError is a module the analysis could not load: a package that does not
// type check, or a loader that failed.
type LoadError struct {
	// Root is the module root the load ran in.
	Root string

	// cause is what the loader said.
	cause error
}

func (e *LoadError) Error() string {
	if e.cause == nil {
		return Name + ": loading " + e.Root + " failed"
	}
	return Name + ": loading " + e.Root + ": " + e.cause.Error()
}

// Unwrap returns what the loader said.
func (e *LoadError) Unwrap() error {
	return e.cause
}
