// A helper file is the one place a suite file's neighbours may live.
package suitecontents_test

var alwaysTrue = true

// registry is not Ginkgo, so a hook registered through it is not a suite hook
// whatever its method is named.
type registry struct{}

func (registry) BeforeSuite(body func()) bool { body(); return true }

var hooks registry
