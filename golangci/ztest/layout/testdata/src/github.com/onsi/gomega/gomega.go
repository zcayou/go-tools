// Package gomega stands in for Gomega, which is not on a fixture's import path
// since analysistest loads with GOPROXY=off.
package gomega

func RegisterFailHandler(handler func(message string, callerSkip ...int)) {}
