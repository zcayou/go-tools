package zenum

import (
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

const analyzerDoc = `reports enums that do not name themselves, and code that
decides on their rendered names or forges their values

Loads the module once per golangci-lint invocation, sharing the load with
zerrors, and reports each package's share of the result. Whether an enum
is visible, and whether two restate each other, are properties of the whole
module, so the analysis always covers it regardless of the package pattern
given; only reporting is narrowed to it.`

// analyze runs the engine over the module rooted at root. Nothing bounds
// the run, for the reasons [wholemodule.Linter] gives.
func (p *Plugin) analyze(root string) ([]wholemodule.Finding, error) {
	module, err := wholemodule.Load(root, p.buildTags)
	if err != nil {
		return nil, err
	}
	found, err := engine.Analyze(module, p.config)
	if err != nil {
		return nil, err
	}
	findings := make([]wholemodule.Finding, 0, len(found))
	for _, finding := range found {
		findings = append(findings, wholemodule.Finding{
			Pos:      finding.Pos,
			Category: finding.Rule.String(),
			Message:  finding.Message(),
		})
	}
	return findings, nil
}
