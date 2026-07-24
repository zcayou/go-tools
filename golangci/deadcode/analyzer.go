package deadcode

import (
	"context"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

const analyzerDoc = `reports code that nothing in the program can reach

Runs a whole-program call-graph analysis once per golangci-lint invocation and
reports each package's share of the result. Because reachability is a property
of the whole program, the analysis always covers the module regardless of the
package pattern given; only reporting is narrowed to it.`

func (p *Plugin) analyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: Name,
		Doc:  analyzerDoc,
		Run:  p.run,
	}
}

// run reports the findings belonging to this pass's files. The whole-program
// analysis happens once, on whichever pass gets there first; every pass then
// looks up its own files.
func (p *Plugin) run(pass *analysis.Pass) (any, error) {
	p.once.Do(p.analyze)
	if p.err != nil {
		return nil, p.err
	}

	for _, syntax := range pass.Files {
		file := pass.Fset.File(syntax.FileStart)
		if file == nil {
			continue
		}
		for _, finding := range p.byFile[file.Name()] {
			pos, ok := remap(file, finding.Pos)
			if !ok {
				continue
			}
			pass.Report(analysis.Diagnostic{
				Pos:      pos,
				Category: string(finding.Verdict),
				Message:  string(finding.Verdict) + ": " + finding.Name,
			})
		}
	}
	return nil, nil
}

// analyze runs the engine once for the whole invocation. The module root
// is resolved here rather than when the plugin is built, so that a repository
// golangci-lint cannot anchor fails this linter alone instead of every command
// that loads the configuration.
//
// Nothing bounds the run: go/analysis carries no context, golangci-lint offers
// a plugin none of its own, and its run.timeout is a deadline the analysis
// runner discards. A wall-clock bound of this linter's own would turn a slow
// run into a failed one, which is worse than a slow answer.
func (p *Plugin) analyze() {
	dir, err := moduleRoot()
	if err != nil {
		p.err = err
		return
	}
	p.config.Dir = dir

	findings, err := engine.Analyze(context.Background(), p.config)
	if err != nil {
		p.err = err
		return
	}
	p.byFile = make(map[string][]engine.Finding)
	for _, finding := range findings {
		p.byFile[finding.Pos.Filename] = append(p.byFile[finding.Pos.Filename], finding)
	}
}

// remap converts a position the engine resolved in its own FileSet into
// a [token.Pos] in the pass's. Both loads read the same bytes, so the byte
// offset carries over exactly; a mismatch means the file changed underneath us,
// and the finding is dropped rather than reported at a wrong place.
func remap(file *token.File, pos token.Position) (token.Pos, bool) {
	if pos.Offset < 0 || pos.Offset > file.Size() {
		return token.NoPos, false
	}
	return file.Pos(pos.Offset), true
}
