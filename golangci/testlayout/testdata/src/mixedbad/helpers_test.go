package mixedbad

// Ginkgo is not on a fixture's import path, since analysistest loads with
// GOPROXY=off, so the builders the rules match by name are declared here.
// The external test package reaches them through the package under test.
func Describe(text string, body func()) bool { body(); return true }

func It(text string, body func()) bool { body(); return true }
