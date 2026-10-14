package wholemodule_test

import (
	"errors"

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
		Expect(err).To(MatchError("pattern ./internal/... matched only main or internal packages, which are not API: " +
			"example.com/plain/internal/hidden"))
	})

	It("rejects a pattern matching no package", func() {
		_, err := wholemodule.ResolveAPI(fixtureDir("plain"), nil, []string{"./docs/..."})

		Expect(err).To(MatchError(&wholemodule.PatternError{Pattern: "./docs/..."}))
		Expect(err).To(MatchError("pattern ./docs/... matched no packages"))
	})

	It("rejects a pattern matching only packages outside the module", func() {
		_, err := wholemodule.ResolveAPI(fixtureDir("plain"), nil, []string{"./lib", "fmt"})

		Expect(err).To(MatchError(&wholemodule.PatternError{Pattern: "fmt", Foreign: []string{"fmt"}}))
		Expect(err).To(MatchError("pattern fmt matched only packages outside the module: fmt"))
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
