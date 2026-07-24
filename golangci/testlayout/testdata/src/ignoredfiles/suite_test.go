//go:build integration

// No build compiles this file, so the adapter it holds is not in the test
// binary and does not answer for the specs in widget_test.go.
package ignoredfiles_test

import "testing"

func TestIntegration(t *testing.T) {
	RunSpecs(t, "Ignored Files Integration Suite")
}
