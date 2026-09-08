package standalonedoc_test

import "testing"

func Describe(text string, body func()) bool { body(); return true }

func It(text string, body func()) bool { body(); return true }

func RunSpecs(t *testing.T, description string) { t.Log(description) }
