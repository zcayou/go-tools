// Package ginkgo stands in for Ginkgo, which is not on a fixture's import path
// since analysistest loads with GOPROXY=off. It sits at Ginkgo's import path,
// which is what the rules resolve a call against, and declares the part of the
// DSL the fixtures call.
package ginkgo

import "testing"

func Describe(text string, body func()) bool { body(); return true }

func Context(text string, body func()) bool { body(); return true }

func When(text string, body func()) bool { body(); return true }

func It(text string, body func()) bool { body(); return true }

func DescribeTable(text string, body func()) bool { body(); return true }

func DescribeTableSubtree(text string, body func()) bool { body(); return true }

func BeforeSuite(body func()) bool { body(); return true }

func AfterSuite(body func()) bool { body(); return true }

func SynchronizedBeforeSuite(first func() []byte, all func([]byte)) bool { all(first()); return true }

func ReportAfterSuite(text string, body func()) bool { body(); return true }

func Fail(message string, callerSkip ...int) {}

func RunSpecs(t *testing.T, description string) bool { t.Log(description); return true }
