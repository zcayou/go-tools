package pinning

type Failure struct {
	Code int
}

func (f *Failure) Error() string { return "failure" }

func Do() error { return &Failure{Code: 1} }
