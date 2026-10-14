package zerrors

import (
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
	"github.com/zcayou/go-tools/golangci/zerrors/engine"
)

const analyzerDoc = `reports errors a caller cannot tell apart, and decisions on error text

Judges the module's errors once per golangci-lint invocation and reports each
package's share of the result: returns from the API that carry an error no
caller can identify, branches on an error's text, error fields that only ever
hold constants, and error types missing the configured base. What reaches
a return is a property of the whole module, so the analysis always covers it
regardless of the package pattern given; only reporting is narrowed to it.`

// analyze loads the module rooted at root, sharing the load with any other
// linter loading it under the same build tags, and runs the engine over it.
func (p *Plugin) analyze(root string) ([]wholemodule.Finding, error) {
	module, err := wholemodule.Load(root, p.config.BuildTags)
	if err != nil {
		return nil, &LoadError{Root: root, cause: err}
	}
	found, err := engine.Analyze(module, p.config)
	if err != nil {
		return nil, err
	}
	findings := make([]wholemodule.Finding, 0, len(found))
	for _, finding := range found {
		rule := finding.Rule.String()
		findings = append(findings, wholemodule.Finding{
			Pos:      finding.Pos,
			Category: rule,
			Message:  rule + ": " + finding.Message,
		})
	}
	return findings, nil
}
