package engine

import (
	"go/types"

	"golang.org/x/tools/go/packages"
)

// vocabulary is the published names [Config.VocabularyNames] credits:
// the String method of each closed vocabulary the analyzed packages declare.
// A vocabulary is a named type whose own package declares typed constants
// of it, and its name is a String() string method on a value receiver.
// Publishing one name per member is a repository's API decision rather than
// evidence the program carries, so the credit is a setting: a vocabulary
// production compares and never renders keeps its name.
type vocabulary struct {
	// names holds the credited declarations, keyed as participation keys them.
	names map[string]bool
	// methods holds the credited methods themselves, one per package variant
	// declaring them, which the root set takes the way it takes any credited
	// method's.
	methods []*types.Func
}

// newVocabulary finds the published names of the analyzed packages'
// vocabularies, or none while the setting is off. A constant declared
// in a test file makes nothing a vocabulary: the members are what production
// publishes.
func newVocabulary(pkgs []*packages.Package, enabled bool) *vocabulary {
	vocab := &vocabulary{names: map[string]bool{}}
	if !enabled {
		return vocab
	}
	for _, pkg := range pkgs {
		if pkg.Types == nil || synthesizedPackage(pkg.PkgPath) {
			continue
		}
		scope := pkg.Types.Scope()
		seen := map[*types.Named]bool{}
		for _, name := range scope.Names() {
			constant, ok := scope.Lookup(name).(*types.Const)
			if !ok || testFile(position(pkg.Fset, constant.Pos()).Filename) {
				continue
			}
			named, ok := types.Unalias(constant.Type()).(*types.Named)
			if !ok || named.Obj().Pkg() != pkg.Types || seen[named] {
				continue
			}
			seen[named] = true
			if method := publishedName(named); method != nil {
				vocab.names[declKey(position(pkg.Fset, method.Pos()))] = true
				vocab.methods = append(vocab.methods, method)
			}
		}
	}
	return vocab
}

// publishedName returns the vocabulary's String() string method on a value
// receiver, or nil when it declares none.
func publishedName(named *types.Named) *types.Func {
	for method := range named.Methods() {
		if method.Name() != "String" {
			continue
		}
		sig := method.Signature()
		if _, pointer := sig.Recv().Type().(*types.Pointer); pointer {
			return nil
		}
		if sig.Params().Len() != 0 || sig.Results().Len() != 1 ||
			!types.Identical(sig.Results().At(0).Type(), types.Typ[types.String]) {
			return nil
		}
		return method
	}
	return nil
}

func (v *vocabulary) published(key string) bool {
	return v.names[key]
}
