package engine_test

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

// analyze runs the engine over a fixture module and renders each finding
// of one rule as its file, line, rule and message.
func analyze(fixture string, cfg engine.Config, rule engine.Rule) []string {
	GinkgoHelper()

	found, err := run(fixture, cfg)
	Expect(err).NotTo(HaveOccurred())
	var rendered []string
	for _, finding := range found {
		if finding.Rule != rule {
			continue
		}
		rel, err := filepath.Rel(fixtureDir(fixture), finding.Pos.Filename)
		Expect(err).NotTo(HaveOccurred())
		rendered = append(rendered, fmt.Sprintf("%s:%d: %s: %s", rel, finding.Pos.Line, finding.Rule, finding.Message))
	}
	return rendered
}

// run loads a fixture module and runs the engine over it.
func run(fixture string, cfg engine.Config) ([]engine.Finding, error) {
	GinkgoHelper()

	module, err := wholemodule.Load(fixtureDir(fixture), nil)
	Expect(err).NotTo(HaveOccurred())
	return engine.Analyze(module, cfg)
}

func fixtureDir(name string) string {
	GinkgoHelper()

	dir, err := filepath.Abs(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())
	return dir
}
