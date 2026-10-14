package engine

import (
	"go/types"

	"golang.org/x/tools/go/packages"

	"github.com/zcayou/go-tools/golangci/internal/enum"
)

// enumNames is the published names [Config.EnumNames] credits: the String
// method of each enum the analyzed packages declare, as the module's shared
// definition of an enum has it. An enum's name is a String() string method
// on a value receiver. Publishing one name per member is a repository's API
// decision rather than evidence the program carries, so the credit
// is a setting: an enum production compares and never renders keeps its name.
type enumNames struct {
	// names holds the credited declarations, keyed as participation keys them.
	names map[string]bool
	// methods holds the credited methods themselves, one per package variant
	// declaring them, which the root set takes the way it takes any credited
	// method's.
	methods []*types.Func
}

// newEnumNames finds the published names of the analyzed packages' enums,
// or none while the setting is off.
func newEnumNames(pkgs []*packages.Package, enabled bool) *enumNames {
	credit := &enumNames{names: map[string]bool{}}
	if !enabled {
		return credit
	}
	for _, pkg := range pkgs {
		if pkg.Types == nil || synthesizedPackage(pkg.PkgPath) {
			continue
		}
		for _, declared := range enum.Declared(enum.Package{Fset: pkg.Fset, Files: pkg.Syntax, Types: pkg.Types, Info: pkg.TypesInfo}) {
			if method := enum.Name(declared.Type); method != nil {
				credit.names[declKey(position(pkg.Fset, method.Pos()))] = true
				credit.methods = append(credit.methods, method)
			}
		}
	}
	return credit
}

func (v *enumNames) published(key string) bool {
	return v.names[key]
}
