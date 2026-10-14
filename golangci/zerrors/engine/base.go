package engine

import "go/types"

// missingBases reports each module error type that does not carry the base,
// while one is set, in the packages base-packages names. The base itself is not
// judged, nor is an error type in a package the base's own package imports,
// which could not embed the base without an import cycle.
func (a *analysis) missingBases() []Finding {
	if a.base == nil {
		return nil
	}
	below := map[*types.Package]bool{}
	var walk func(*types.Package)
	walk = func(pkg *types.Package) {
		for _, imported := range pkg.Imports() {
			if !below[imported] {
				below[imported] = true
				walk(imported)
			}
		}
	}
	walk(a.base.Obj().Pkg())

	verb := "embed"
	suffix := " by value"
	if types.IsInterface(a.base) {
		verb, suffix = "implement", ""
	}
	var findings []Finding
	for _, named := range a.idx.types {
		if named == a.base.Origin() || errorNamed(named) == nil || below[named.Obj().Pkg()] || a.carries(named) {
			continue
		}
		if a.basePackages != nil && !a.basePackages[named.Obj().Pkg().Path()] {
			continue
		}
		subject := typeName(named)
		findings = append(findings, Finding{
			Pos:     a.module.Fset.PositionFor(named.Obj().Pos(), false),
			Rule:    RuleMissingBase,
			Subject: subject,
			Message: subject + " does not " + verb + " " + typeName(a.base) + suffix,
		})
	}
	return findings
}
