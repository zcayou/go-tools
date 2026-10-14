package pinning

// Report's Error takes an argument, so it is not the method an error type
// renders through.
type Report struct{}

func (Report) Error(code int) string { return "" }
