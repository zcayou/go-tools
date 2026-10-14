package engine

import (
	"errors"
	"go/types"
	"slices"
	"strings"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// analysis is one run: the module, the configuration resolved against it,
// and the index every rule reads.
type analysis struct {
	module *wholemodule.Module
	prog   *wholemodule.Program
	cfg    Config
	idx    *index

	// api holds the import paths of the API packages.
	api map[string]bool

	// aliased holds the types an exported alias of an API package names.
	aliased map[*types.Named]bool

	// base is the resolved base, or nil while missing-base is off.
	base *types.Named

	// basePackages holds the import paths of the packages whose error types
	// missing-base judges, or nil for every module package.
	basePackages map[string]bool

	protocols []protocol
}

// protocol is one resolved protocols entry.
type protocol struct {
	entry  string
	iface  *types.Named
	method *types.Func
}

func newAnalysis(module *wholemodule.Module, cfg Config) (*analysis, error) {
	api, err := wholemodule.ResolveAPI(module.Root, cfg.BuildTags, cfg.API)
	if err != nil {
		return nil, unmatched(SettingAPI, err)
	}

	a := &analysis{module: module, cfg: cfg, api: api}
	a.aliased = a.exportedAliases()

	if err := a.checkTrusted(); err != nil {
		return nil, err
	}
	if err := a.resolveBase(); err != nil {
		return nil, err
	}
	if err := a.resolveBasePackages(); err != nil {
		return nil, err
	}
	a.prog = module.Program()
	a.idx = newIndex(a)
	if err := a.resolveProtocols(); err != nil {
		return nil, err
	}
	return a, nil
}

// unmatched is a setting's package pattern that resolving it, which failed
// with err, found no package for.
func unmatched(setting Setting, err error) *ConfigError {
	value := ""
	if pattern, ok := errors.AsType[*wholemodule.PatternError](err); ok {
		value = pattern.Pattern
	}
	return &ConfigError{Setting: setting, Problem: ProblemMatchesNothing, Value: value, cause: err}
}

// checkTrusted fails on a trusted entry matching no foreign package the module
// imports, directly or transitively: trusting a package the module never
// reaches says nothing.
func (a *analysis) checkTrusted() error {
	for _, entry := range a.cfg.Trusted {
		if !slices.ContainsFunc(a.module.ImportedPaths(), func(path string) bool { return trustedBy(entry, path) }) {
			return &ConfigError{Setting: SettingTrusted, Problem: ProblemMatchesNothing, Value: entry}
		}
	}
	return nil
}

// trustedBy reports whether a trusted entry names the package at path.
func trustedBy(entry, path string) bool {
	if prefix, tree := strings.CutSuffix(entry, "/..."); tree {
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	return path == entry
}

// trusted reports whether pkg is foreign and named by a trusted entry.
func (a *analysis) trusted(pkg *types.Package) bool {
	if pkg == nil || a.module.Package(pkg) != nil {
		return false
	}
	for _, entry := range a.cfg.Trusted {
		if trustedBy(entry, pkg.Path()) {
			return true
		}
	}
	return false
}

func (a *analysis) resolveBase() error {
	if a.cfg.Base == "" {
		return nil
	}
	obj := a.lookup(a.cfg.Base)
	if obj == nil {
		return &ConfigError{Setting: SettingBase, Problem: ProblemUnresolved, Value: a.cfg.Base}
	}
	named, ok := types.Unalias(obj.Type()).(*types.Named)
	if !ok {
		return &ConfigError{Setting: SettingBase, Problem: ProblemWrongKind, Value: a.cfg.Base}
	}
	switch named.Underlying().(type) {
	case *types.Struct, *types.Interface:
		a.base = named
		return nil
	default:
		return &ConfigError{Setting: SettingBase, Problem: ProblemWrongKind, Value: a.cfg.Base}
	}
}

// resolveBasePackages resolves the packages missing-base judges, while a base
// is set and base-packages narrows them.
func (a *analysis) resolveBasePackages() error {
	if a.base == nil || a.cfg.BasePackages == nil {
		return nil
	}
	judged, err := wholemodule.ResolvePackages(a.module.Root, a.cfg.BuildTags, a.cfg.BasePackages)
	if err != nil {
		return unmatched(SettingBasePackages, err)
	}
	a.basePackages = judged
	return nil
}

// lookup resolves import/path.Name to the type it names in a package the module
// declares or imports, or nil.
func (a *analysis) lookup(qualified string) *types.TypeName {
	path, name, ok := strings.CutLast(qualified, ".")
	if !ok {
		return nil
	}
	pkg := a.module.ImportedPackage(path)
	if pkg == nil {
		return nil
	}
	obj, _ := pkg.Scope().Lookup(name).(*types.TypeName)
	return obj
}

// resolveProtocols resolves each entry to its interface method. Whether one
// exempts anything is known only once the judged set is, so judgedFunctions
// checks that.
func (a *analysis) resolveProtocols() error {
	for _, entry := range a.cfg.Protocols {
		unresolved := &ConfigError{Setting: SettingProtocols, Problem: ProblemUnresolved, Value: entry}
		qualified, name, ok := strings.CutLast(entry, ".")
		if !ok {
			return unresolved
		}
		obj := a.lookup(qualified)
		if obj == nil {
			return unresolved
		}
		named, ok := types.Unalias(obj.Type()).(*types.Named)
		if !ok {
			return unresolved
		}
		iface, ok := named.Underlying().(*types.Interface)
		if !ok {
			return unresolved
		}
		var method *types.Func
		for m := range iface.Methods() {
			if m.Name() == name {
				method = m
			}
		}
		if method == nil {
			return unresolved
		}
		a.protocols = append(a.protocols, protocol{entry: entry, iface: named, method: method})
	}
	return nil
}
