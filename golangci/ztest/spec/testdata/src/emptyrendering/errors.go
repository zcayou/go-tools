package emptyrendering

type Failure struct{}

func (f *Failure) Error() string { return "failure" }

func Do() error { return &Failure{} }
