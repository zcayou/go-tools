package engine

import (
	"cmp"
	"errors"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/internal/enum"
	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// Config is what the analysis needs beyond the module. Each list names types
// as import/path.Name.
type Config struct {
	// OpenEnums are types whose values come from outside the module. They are no
	// enum to any rule.
	OpenEnums []string

	// ZeroDefaults are enums whose zero member is the documented default,
	// which zero-member then leaves alone.
	ZeroDefaults []string

	// RelatedEnums are pairs whose tokens overlap because they classify different
	// things, which restated then leaves alone.
	RelatedEnums []Pair
}

// Pair is two enums, unordered.
type Pair struct {
	First  string
	Second string
}

// Finding is one diagnostic.
type Finding struct {
	// Pos is where the finding sits. It is a resolved [token.Position], without
	// //line adjustment, so that it outlives the module's [token.FileSet]
	// and names the file on disk.
	Pos token.Position

	// Rule is what the finding reports.
	Rule Rule

	// Enum is the enum the finding is about, as import/path.Name.
	Enum string

	// Detail is the message after the rule token.
	Detail string
}

// Message is the whole diagnostic text, opening with the rule's token.
func (f Finding) Message() string {
	return f.Rule.String() + ": " + f.Detail
}

// Analyze judges the enums the module declares, and every use of them in module
// code. Findings are sorted by file and offset.
//
// An exception in cfg that suppresses nothing fails the analysis
// with an [*ExceptionError], or several joined, and no findings.
func Analyze(module *wholemodule.Module, cfg Config) ([]Finding, error) {
	a := newAnalysis(module)
	stale := a.exceptions(cfg)
	functions := a.functions()
	a.see(functions)
	a.declarations()
	stale = append(stale, a.restatements(cfg.RelatedEnums)...)
	a.renderings(functions)
	a.syntax()
	a.instantiated(functions)

	if len(stale) == 1 {
		return nil, stale[0]
	}
	if len(stale) > 1 {
		return nil, errors.Join(stale...)
	}
	slices.SortFunc(a.findings, func(x, y Finding) int {
		return cmp.Or(
			cmp.Compare(x.Pos.Filename, y.Pos.Filename),
			cmp.Compare(x.Pos.Offset, y.Pos.Offset),
			cmp.Compare(x.Rule, y.Rule),
			cmp.Compare(x.Detail, y.Detail),
		)
	})
	return a.findings, nil
}

// analysis is one run over a module.
type analysis struct {
	module *wholemodule.Module

	// enums are the enums the rules judge, in import-path-then-name order.
	enums []*enumType

	// byType and byName index enums by their type and by their String method.
	byType map[*types.Named]*enumType
	byName map[*types.Func]*enumType

	// carriage memoizes what each type carries.
	carriage *carriage

	// generic holds each conversion to a type parameter the syntax writes,
	// by the position of its opening parenthesis, which is where an SSA conversion
	// instruction says it was written.
	generic map[token.Pos]genericConversion

	findings []Finding
	reported map[Finding]bool
}

// newAnalysis finds the module's enums, before any exception takes one
// out.
func newAnalysis(module *wholemodule.Module) *analysis {
	a := &analysis{
		module:   module,
		byType:   map[*types.Named]*enumType{},
		byName:   map[*types.Func]*enumType{},
		carriage: newCarriage(),
		generic:  map[token.Pos]genericConversion{},
		reported: map[Finding]bool{},
	}
	pkgs := slices.Clone(module.Packages)
	slices.SortFunc(pkgs, func(x, y *packages.Package) int { return cmp.Compare(x.PkgPath, y.PkgPath) })
	for _, pkg := range pkgs {
		for _, declared := range enum.Declared(enum.Package{Fset: pkg.Fset, Files: pkg.Syntax, Types: pkg.Types, Info: pkg.TypesInfo}) {
			if !declared.Ordinal() && !declared.Textual() {
				continue
			}
			a.enums = append(a.enums, &enumType{
				Enum:      declared,
				pkg:       pkg,
				qualified: pkg.PkgPath + "." + declared.Type.Obj().Name(),
				tokens:    declared.Tokens(),
			})
		}
	}
	return a
}

// report records a finding once. A generic body is read once as written
// and once per instantiation, and each reading finds the same thing.
func (a *analysis) report(pos token.Pos, rule Rule, v *enumType, detail string) {
	finding := Finding{
		Pos:    a.module.Fset.PositionFor(pos, false),
		Rule:   rule,
		Enum:   v.qualified,
		Detail: detail,
	}
	if !finding.Pos.IsValid() || a.reported[finding] {
		return
	}
	a.reported[finding] = true
	a.findings = append(a.findings, finding)
}
