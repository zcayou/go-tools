package suitecontents_test

import "testing"

// Ginkgo is not on a fixture's import path, since analysistest loads with
// GOPROXY=off, so the builders and hooks the rules match by name are declared
// here. A helper file is the one place a suite file's neighbours may live.
func BeforeSuite(body func()) bool { body(); return true }

func AfterSuite(body func()) bool { body(); return true }

func SynchronizedBeforeSuite(first func() []byte, all func([]byte)) bool { all(first()); return true }

func ReportAfterSuite(text string, body func()) bool { body(); return true }

func Describe(text string, body func()) bool { body(); return true }

func RegisterFailHandler(handler any) {}

func Fail(message string, callerSkip ...int) {}

func RunSpecs(t *testing.T, description string) { t.Log(description) }

var alwaysTrue = true
