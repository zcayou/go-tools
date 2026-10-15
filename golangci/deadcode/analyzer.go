package deadcode

import (
	"context"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

const analyzerDoc = `reports code that nothing in the program can reach

Runs a whole-program call-graph analysis once per golangci-lint invocation and
reports each package's share of the result. Because reachability is a property
of the whole program, the analysis always covers the module regardless of the
package pattern given; only reporting is narrowed to it.`

// analyze runs the engine over the module rooted at root. Nothing bounds
// the run, for the reasons [wholemodule.Linter] gives.
func (p *Plugin) analyze(root string) ([]wholemodule.Finding, error) {
	config := p.config
	config.Dir = root

	found, err := engine.Analyze(context.Background(), config)
	if err != nil {
		return nil, err
	}
	findings := make([]wholemodule.Finding, 0, len(found))
	for _, finding := range found {
		findings = append(findings, wholemodule.Finding{
			Pos:      finding.Pos,
			Category: finding.Category(),
			Message:  finding.Message(),
		})
	}
	return findings, nil
}
