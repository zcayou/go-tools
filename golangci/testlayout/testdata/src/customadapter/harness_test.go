// adapter-patterns names this file rather than suite_test.go, so the adapter
// belongs here and the suite-file rules apply here.
package customadapter_test

import "testing"

var _ = BeforeSuite(func() {})

func TestCustomAdapter(t *testing.T) {
	RunSpecs(t, "Custom Adapter Suite")
}

const unwanted = 1 // want `suite-file-contents: a constant declaration; a suite file holds`
