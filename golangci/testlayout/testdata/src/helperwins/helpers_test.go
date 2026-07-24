// helpers.go sits beside this file, so helpers_test.go matches <source>_test.go
// as well as the helper pattern. The helper pattern is consulted first, so the
// file owes no specs.
package helperwins_test

type fakeClient struct{}

// Test is a method, and Testify only shares a prefix with a go test entry
// point, so neither makes this a file carrying specs.
func (fakeClient) Test() error { return nil }

func Testify() int { return 1 }
