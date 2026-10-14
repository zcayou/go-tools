// Package wholemodule fits a linter that judges the whole module at once into
// golangci-lint's per-package contract, and holds the typed load such linters
// share.
//
// go/analysis hands a linter one package at a time, and its facts flow only
// from a package to its importers. What a whole-module linter judges flows
// the other way — implementations, stores and conversions live in importers —
// so the linter does its own load and analysis once, and each pass reports
// the share of the result that falls in the files it holds. Three consequences
// follow, and [Linter] carries all three:
//
//   - The analysis runs once, behind a [sync.Once], on whichever pass gets
//     there first. Its findings are indexed by file, and a finding in a file no
//     pass holds is dropped, which is what scopes reporting to the packages
//     golangci-lint was asked to lint.
//   - The analysis reads its own [token.FileSet], not the pass's, so
//     a finding's position is carried over by byte offset.
//   - golangci-lint caches issues per package, keyed on the package and its
//     imports. A whole-module result changes with the package's importers, so
//     the linter returns a second analyzer whose name is unique to the process,
//     which forces that cache to miss.
package wholemodule

import (
	"crypto/rand"
	"fmt"
	"go/token"
	"sync"

	"golang.org/x/tools/go/analysis"
)

// Finding is one diagnostic a whole-module analysis produced.
type Finding struct {
	// Pos is where the finding sits, resolved in the analysis's own file set
	// without //line adjustment, so its file name is the one on disk.
	Pos token.Position

	// Category is the rule the finding came from: the token golangci-lint users
	// exclude it by.
	Category string

	// Message is the whole diagnostic text. It opens with Category, because
	// golangci-lint discards [analysis.Diagnostic.Category] and the message
	// is what an exclusion rule matches.
	Message string
}

// Analyze runs a whole-module analysis over the module rooted at root.
type Analyze func(root string) ([]Finding, error)

// Linter adapts one [Analyze] to golangci-lint. It holds the single analysis
// the whole run shares.
type Linter struct {
	name    string
	doc     string
	analyze Analyze

	once   sync.Once
	byFile map[string][]Finding
	err    error
}

// New adapts analyze to golangci-lint under the linter's name and doc.
func New(name, doc string, analyze Analyze) *Linter {
	return &Linter{name: name, doc: doc, analyze: analyze}
}

// Analyzers returns the analyzer that reports this linter's findings,
// alongside the one that keeps them from being cached. It is what the linter's
// BuildAnalyzers returns.
func (l *Linter) Analyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		{Name: l.name, Doc: l.doc, Run: l.run},
		uncacheable(l.name),
	}
}

// run reports the findings belonging to this pass's files. A failed analysis
// fails every pass: golangci-lint keeps the first error and skips caching
// on any, so reporting the failure once and staying silent afterwards would
// gain nothing.
func (l *Linter) run(pass *analysis.Pass) (any, error) {
	l.once.Do(l.analyzeOnce)
	if l.err != nil {
		return nil, l.err
	}

	for _, syntax := range pass.Files {
		file := pass.Fset.File(syntax.FileStart)
		if file == nil {
			continue
		}
		for _, finding := range l.byFile[file.Name()] {
			pos, ok := remap(file, finding.Pos)
			if !ok {
				continue
			}
			pass.Report(analysis.Diagnostic{Pos: pos, Category: finding.Category, Message: finding.Message})
		}
	}
	return nil, nil
}

// analyzeOnce runs the analysis for the whole invocation. The module root
// is resolved here rather than when the plugin is built, so that a repository
// golangci-lint cannot anchor fails this linter alone instead of every command
// that loads the configuration.
//
// Nothing bounds the run: go/analysis carries no context, golangci-lint offers
// a plugin none of its own, and its run.timeout is a deadline the analysis
// runner discards. A wall-clock bound of the linter's own would turn a slow
// run into a failed one, which is worse than a slow answer.
func (l *Linter) analyzeOnce() {
	root, err := ModuleRoot()
	if err != nil {
		l.err = err
		return
	}
	findings, err := l.analyze(root)
	if err != nil {
		l.err = err
		return
	}
	l.byFile = make(map[string][]Finding)
	for _, finding := range findings {
		l.byFile[finding.Pos.Filename] = append(l.byFile[finding.Pos.Filename], finding)
	}
}

// remap converts a position the analysis resolved in its own FileSet into
// a [token.Pos] in the pass's. Both loads read the same bytes, so the byte
// offset carries over exactly; a mismatch means the file changed underneath us,
// and the finding is dropped rather than reported at a wrong place.
func remap(file *token.File, pos token.Position) (token.Pos, bool) {
	if pos.Offset < 0 || pos.Offset > file.Size() {
		return token.NoPos, false
	}
	return file.Pos(pos.Offset), true
}

// uncacheable defeats golangci-lint's per-package issue cache. That cache keys
// on a package and its imports, while a whole-module result flows the other
// way, from importers: a package whose own hash is unchanged would replay
// an answer the edit that changed it never invalidated, silently keeping
// a finding that is now wrong or losing one that is now right. golangci-lint
// folds every analyzer name into the cache key, so a name unique to this
// process forces a miss for every package.
func uncacheable(name string) *analysis.Analyzer {
	var nonce [8]byte
	_, _ = rand.Read(nonce[:])
	return &analysis.Analyzer{
		Name: fmt.Sprintf("%s_uncacheable_%x", name, nonce),
		Doc:  "forces a cache miss; reports nothing",
		Run:  func(*analysis.Pass) (any, error) { return nil, nil },
	}
}
