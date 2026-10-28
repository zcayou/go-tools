package engine

import (
	"cmp"
	"errors"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/objectpath"

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

	// OrderedEnums are enums whose ordinals are an order, which ordered-comparison
	// and member-bound then leave alone.
	OrderedEnums []string

	// Tests has the use-site rules judge test files too: each package's in-package
	// and external tests.
	Tests bool

	// OrderedInstantiations has ordered-comparison judge an instantiation
	// passing an enum to a type parameter whose constraint orders its values,
	// as a sort's does.
	OrderedInstantiations bool

	// Disabled are rules that report nothing. Every rule runs by default.
	Disabled []Rule
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
	a := newAnalysis(module, cfg)
	stale := a.exceptions(cfg)
	plain := a.plainCorpus()
	plain.inferPrinters()
	a.see(plain.functions)
	a.declarations()
	stale = append(stale, a.restatements(cfg.RelatedEnums)...)
	corpora := []*corpus{plain}
	if cfg.Tests {
		tests := a.testCorpus(plain)
		tests.inferPrinters()
		corpora = append(corpora, tests)
	}
	for _, c := range corpora {
		a.renderings(c.functions)
		c.syntax()
	}
	stale = append(stale, a.unordered(cfg)...)
	for _, c := range corpora {
		a.instantiated(c.functions)
		a.instantiatedFormats(c.functions)
	}

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

	// pkgs are the module's packages in import-path order.
	pkgs []*packages.Package

	// enums are the enums the rules judge, in import-path-then-name order.
	enums []*enumType

	// byType and byName index enums by their type and by their String method.
	byType map[*types.Named]*enumType
	byName map[*types.Func]*enumType

	// plain holds the module's packages by import path, and canonicals what
	// [analysis.canonical] has mapped, through paths.
	plain      map[string]*packages.Package
	canonicals map[types.Object]types.Object
	paths      *objectpath.Encoder

	// carriage memoizes what each type carries.
	carriage *carriage

	// generic holds each conversion to or from a type parameter the syntax writes,
	// by the position of its opening parenthesis, which is where an SSA conversion
	// instruction says it was written.
	generic map[token.Pos]genericConversion

	// modulePaths are the import paths of the module's packages and of their
	// test packages.
	modulePaths map[string]bool

	// genericFormats holds each printer's call in a generic body formatting
	// a type parameter's value, by the position of its opening parenthesis,
	// which is where an SSA call instruction says it was written.
	genericFormats map[token.Pos]genericFormat

	// orderedInstantiations is [Config.OrderedInstantiations].
	orderedInstantiations bool

	disabled []Rule
	findings []Finding
	reported map[Finding]bool
}

// newAnalysis finds the module's enums, before any exception takes one
// out.
func newAnalysis(module *wholemodule.Module, cfg Config) *analysis {
	a := &analysis{
		module:                module,
		modulePaths:           map[string]bool{},
		byType:                map[*types.Named]*enumType{},
		byName:                map[*types.Func]*enumType{},
		plain:                 map[string]*packages.Package{},
		canonicals:            map[types.Object]types.Object{},
		paths:                 new(objectpath.Encoder),
		carriage:              newCarriage(),
		generic:               map[token.Pos]genericConversion{},
		genericFormats:        map[token.Pos]genericFormat{},
		orderedInstantiations: cfg.OrderedInstantiations,
		disabled:              cfg.Disabled,
		reported:              map[Finding]bool{},
	}
	a.pkgs = slices.Clone(module.Packages)
	slices.SortFunc(a.pkgs, func(x, y *packages.Package) int { return cmp.Compare(x.PkgPath, y.PkgPath) })
	for _, pkg := range module.Tests {
		a.modulePaths[pkg.PkgPath] = true
	}
	for _, pkg := range a.pkgs {
		a.modulePaths[pkg.PkgPath] = true
		a.plain[pkg.PkgPath] = pkg
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

// enabled reports whether rule reports what it finds.
func (a *analysis) enabled(rule Rule) bool {
	return !slices.Contains(a.disabled, rule)
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
	if !finding.Pos.IsValid() || a.reported[finding] || !a.enabled(rule) {
		return
	}
	a.reported[finding] = true
	a.findings = append(a.findings, finding)
}
