package lib

import "testing"

func TestCaller(t *testing.T) {
	var caller Caller
	_ = caller.Fetch[int]()
	caller.Direct[string]()
	caller.Plain()
	worker{}.Run[int]()
}
