package mixed

// Ginkgo is not on a fixture's import path, since analysistest loads with
// GOPROXY=off, so the builders the rules match by name are declared here.
func Describe(text string, body func()) bool { body(); return true }

func It(text string, body func()) bool { body(); return true }
