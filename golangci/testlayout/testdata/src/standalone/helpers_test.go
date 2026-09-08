package standalone

import "testing"

// Ginkgo is not on a fixture's import path, since analysistest loads with
// GOPROXY=off, so the builders the rules match by name are declared here.
func Describe(text string, body func()) bool { body(); return true }

func It(text string, body func()) bool { body(); return true }

func BeforeSuite(body func()) bool { body(); return true }

func AfterSuite(body func()) bool { body(); return true }

func RunSpecs(t *testing.T, description string) { t.Log(description) }
