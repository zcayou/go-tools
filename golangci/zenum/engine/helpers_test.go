package engine_test

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

// Each fixture under testdata is its own module, loaded the way the linter
// loads one, so a spec sees exactly the module the fixture describes. Loads
// are memoized for the process, so fixtures shared between specs load once.

// analyzeFixture runs the engine over one fixture module and renders each
// finding as its file, line and column, its rule, and the enum it is about.
func analyzeFixture(name string, cfg engine.Config) []string {
	GinkgoHelper()

	dir := fixtureDir(name)
	findings, err := engine.Analyze(load(dir), cfg)
	Expect(err).NotTo(HaveOccurred())

	var reported []string //nolint:prealloc // a fixture reporting nothing is asserted as []string(nil).
	for _, finding := range findings {
		reported = append(reported, fmt.Sprintf("%s: %s: %s", where(dir, finding), finding.Rule, finding.Enum))
	}
	return reported
}

// messages runs the engine over one fixture module and returns each finding's
// whole message by where it sits.
func messages(name string, cfg engine.Config) map[string]string {
	GinkgoHelper()

	dir := fixtureDir(name)
	findings, err := engine.Analyze(load(dir), cfg)
	Expect(err).NotTo(HaveOccurred())

	byPosition := map[string]string{}
	for _, finding := range findings {
		byPosition[where(dir, finding)] = finding.Message()
	}
	return byPosition
}

// analyzeError runs the engine over one fixture module where cfg is expected
// to fail it.
func analyzeError(name string, cfg engine.Config) error {
	GinkgoHelper()

	findings, err := engine.Analyze(load(fixtureDir(name)), cfg)
	Expect(findings).To(BeEmpty())
	return err
}

func where(dir string, finding engine.Finding) string {
	GinkgoHelper()

	rel, err := filepath.Rel(dir, finding.Pos.Filename)
	Expect(err).NotTo(HaveOccurred())
	return fmt.Sprintf("%s:%d:%d", rel, finding.Pos.Line, finding.Pos.Column)
}

func load(dir string) *wholemodule.Module {
	GinkgoHelper()

	module, err := wholemodule.Load(dir, nil)
	Expect(err).NotTo(HaveOccurred())
	return module
}

func fixtureDir(name string) string {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())
	return dir
}
