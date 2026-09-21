package suitecontents_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = BeforeSuite(func() {})

var _ = AfterSuite(func() {})

var _ = SynchronizedBeforeSuite(func() []byte { return nil }, func(_ []byte) {})

var _ = ReportAfterSuite("report", func() {})

func TestSuiteContents(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Suite Contents Suite")
}

func TestMain(m *testing.M) { m.Run() }

const unwantedConst = 1 // want `suite-file-contents: a constant declaration; a suite file holds`

type unwantedType struct{} // want `suite-file-contents: a type declaration; a suite file holds`

var unwantedVar = 2 // want `suite-file-contents: a variable declaration; a suite file holds`

var _ = alwaysTrue // want `suite-file-contents: a variable declaration; a suite file holds`

var _ = hooks.BeforeSuite(func() {}) // want `suite-file-contents: a variable declaration; a suite file holds`

var _ = func() bool { return true }() // want `suite-file-contents: a variable declaration; a suite file holds`

var _ = Describe("a spec belonging beside its source", func() {}) // want `suite-file-contents: a spec; a suite file holds`

func unwantedFunc() int { return 3 } // want `suite-file-contents: a function; a suite file holds`

func (unwantedType) unwantedMethod() {} // want `suite-file-contents: a method; a suite file holds`

// Testify only shares a prefix with a go test entry point, so the file it names
// has no entry point at all.
func Testify(t *testing.T) {} // want `suite-file-contents: a function; a suite file holds`

// These two carry an entry point's name over a signature go test would not run,
// which is what makes go list refuse to build the test binary for this fixture.
func TestTakingAString(s string) {} // want `suite-file-contents: a function; a suite file holds`

func TestTakingSomethingLocal(t *unwantedType) {} // want `suite-file-contents: a function; a suite file holds`
