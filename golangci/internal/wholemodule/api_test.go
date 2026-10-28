package wholemodule_test

import (
	"errors"
	"go/token"
	"go/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

var _ = Describe("ResolveAPI", func() {
	It("takes every package but main and internal ones when no pattern is given", func() {
		api, err := wholemodule.ResolveAPI(fixtureDir("plain"), nil, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(api).To(Equal(map[string]bool{"example.com/plain/lib": true}))
	})

	It("narrows to the packages the patterns match", func() {
		api, err := wholemodule.ResolveAPI(fixtureDir("plain"), nil, []string{"./lib"})

		Expect(err).NotTo(HaveOccurred())
		Expect(api).To(Equal(map[string]bool{"example.com/plain/lib": true}))
	})

	It("makes no internal package API, however explicitly a pattern names it", func() {
		_, err := wholemodule.ResolveAPI(fixtureDir("plain"), nil, []string{"./lib", "./internal/..."})

		Expect(err).To(MatchError(&wholemodule.PatternError{
			Pattern:  "./internal/...",
			Excluded: []string{"example.com/plain/internal/hidden"},
		}))
	})

	It("rejects a pattern matching no package", func() {
		_, err := wholemodule.ResolveAPI(fixtureDir("plain"), nil, []string{"./docs/..."})

		Expect(err).To(MatchError(&wholemodule.PatternError{Pattern: "./docs/..."}))
	})

	It("rejects a pattern matching only packages outside the module", func() {
		_, err := wholemodule.ResolveAPI(fixtureDir("plain"), nil, []string{"./lib", "fmt"})

		Expect(err).To(MatchError(&wholemodule.PatternError{Pattern: "fmt", Foreign: []string{"fmt"}}))
	})

	It("takes no package outside the module into the API, whatever pattern matches it", func() {
		api, err := wholemodule.ResolveAPI(fixtureDir("plain"), nil, []string{"all"})

		Expect(err).NotTo(HaveOccurred())
		Expect(api).To(Equal(map[string]bool{"example.com/plain/lib": true}))
	})

	It("rejects a pattern the loader cannot resolve, carrying what it said", func() {
		_, err := wholemodule.ResolveAPI(fixtureDir("plain"), nil, []string{"./absent"})

		Expect(err).To(BeAssignableToTypeOf(&wholemodule.PatternError{}))
		Expect(err).To(HaveField("Pattern", "./absent"))
		Expect(errors.Unwrap(err)).To(MatchError(ContainSubstring("absent")))
		Expect(err).To(MatchError(HavePrefix("resolving pattern ./absent: ")))
	})
})

var _ = Describe("PatternError", func() {
	DescribeTable("renders the pattern and what it matched",
		func(err *wholemodule.PatternError, message string) {
			Expect(err).To(MatchError(message))
		},
		Entry("only main or internal packages",
			&wholemodule.PatternError{
				Pattern: "./internal/...", Excluded: []string{"example.com/m/internal/a", "example.com/m/cmd"},
			},
			"pattern ./internal/... matched only main or internal packages, which are not API: "+
				"example.com/m/internal/a, example.com/m/cmd"),
		Entry("only packages outside the module",
			&wholemodule.PatternError{Pattern: "all", Foreign: []string{"fmt", "os"}},
			"pattern all matched only packages outside the module: fmt, os"),
		Entry("nothing", &wholemodule.PatternError{Pattern: "./docs/..."}, "pattern ./docs/... matched no packages"),
	)
})

var _ = Describe("ResolvePackages", func() {
	It("takes every package of the module, main and internal ones included, when no pattern is given", func() {
		pkgs, err := wholemodule.ResolvePackages(fixtureDir("plain"), nil, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(pkgs).To(Equal(map[string]bool{
			"example.com/plain":                 true,
			"example.com/plain/lib":             true,
			"example.com/plain/internal/hidden": true,
		}))
	})

	It("narrows to the packages the patterns match, an internal one included", func() {
		pkgs, err := wholemodule.ResolvePackages(fixtureDir("plain"), nil, []string{"./internal/..."})

		Expect(err).NotTo(HaveOccurred())
		Expect(pkgs).To(Equal(map[string]bool{"example.com/plain/internal/hidden": true}))
	})

	It("rejects a pattern matching no package of the module", func() {
		_, err := wholemodule.ResolvePackages(fixtureDir("plain"), nil, []string{"./lib", "fmt"})

		Expect(err).To(MatchError(&wholemodule.PatternError{Pattern: "fmt", Foreign: []string{"fmt"}}))
	})
})

var _ = Describe("Public", func() {
	It("is every package another module can import", func() {
		Expect(wholemodule.Public("example.com/m/lib", "lib")).To(BeTrue())
		Expect(wholemodule.Public("example.com/m/cmd/tool", "main")).To(BeFalse())
		Expect(wholemodule.Public("example.com/m/internal/x", "x")).To(BeFalse())
		Expect(wholemodule.Public("example.com/m/a/internal", "internal")).To(BeFalse())
	})
})

var _ = Describe("Closed", func() {
	declare := func(path, name, typeName string, methods ...string) *types.Named {
		pkg := types.NewPackage(path, name)
		named := types.NewNamed(types.NewTypeName(token.NoPos, pkg, typeName, nil), nil, nil)
		funcs := make([]*types.Func, 0, len(methods))
		for _, method := range methods {
			sig := types.NewSignatureType(types.NewVar(token.NoPos, pkg, "", named), nil, nil, nil, nil, false)
			funcs = append(funcs, types.NewFunc(token.NoPos, pkg, method, sig))
		}
		named.SetUnderlying(types.NewInterfaceType(funcs, nil).Complete())
		return named
	}

	It("is open only when exported from an importable package with exported methods alone", func() {
		Expect(wholemodule.Closed(declare("example.com/m/lib", "lib", "Sink", "Emit"))).To(BeFalse())
		Expect(wholemodule.Closed(declare("example.com/m/lib", "lib", "sink", "Emit"))).To(BeTrue())
		Expect(wholemodule.Closed(declare("example.com/m/internal/x", "x", "Sink", "Emit"))).To(BeTrue())
		Expect(wholemodule.Closed(declare("example.com/m/cmd/tool", "main", "Sink", "Emit"))).To(BeTrue())
		Expect(wholemodule.Closed(declare("example.com/m/lib", "lib", "Sink", "Emit", "seal"))).To(BeTrue())
	})
})
