package engine

import (
	"cmp"
	"go/token"
	"slices"

	"github.com/zcayou/go-tools/golangci/internal/wholemodule"
)

// Config is what the analysis needs. The zero value judges every non-main,
// non-internal package as API, trusts nothing, exempts nothing, counts no
// sentinel as structured, and leaves missing-base off.
type Config struct {
	// API are package patterns, resolved against the module root, naming the API
	// packages. nil means ./... Each pattern has to match an API package.
	API []string

	// Trusted are import paths, each optionally ending in /..., whose errors
	// count as structured and whose interfaces a module type answers to. Each
	// has to match a package the module imports.
	Trusted []string

	// Protocols are interface methods, written import/path.Interface.Method, whose
	// implementations unstructured-return does not judge. Each has to name
	// an interface method and take some judged method out of judgment.
	Protocols []string

	// Sentinels counts an exported package-level error variable of an API
	// package as structured.
	Sentinels bool

	// Base is the import/path.Name of the struct every error type embeds by value,
	// or the interface every error type implements. Empty leaves missing-base off.
	Base string

	// BasePackages are package patterns, resolved against the module root, naming
	// the packages whose error types missing-base judges, main and internal ones
	// included. nil means ./... Each pattern has to match a package of the module.
	BasePackages []string

	// BuildTags are the build tags the module was loaded under, which the api
	// patterns resolve under too.
	BuildTags []string
}

// Finding is one diagnostic.
type Finding struct {
	// Pos is where the finding sits, without //line adjustment.
	Pos token.Position

	// Rule is the rule the finding comes from.
	Rule Rule

	// Subject names what the finding is about: the judged function, the field,
	// the error type, or the function a decision sits in.
	Subject string

	// Message is the diagnostic text that follows the rule's token.
	Message string
}

// Analyze judges the module's errors under cfg. It fails with a [*ConfigError]
// when cfg names something the module does not have.
func Analyze(module *wholemodule.Module, cfg Config) ([]Finding, error) {
	a, err := newAnalysis(module, cfg)
	if err != nil {
		return nil, err
	}

	findings, err := a.unstructuredReturns()
	if err != nil {
		return nil, err
	}
	findings = append(findings, a.textDecisions()...)
	findings = append(findings, a.literalFields()...)
	findings = append(findings, a.missingBases()...)

	slices.SortFunc(findings, func(x, y Finding) int {
		return cmp.Or(
			cmp.Compare(x.Pos.Filename, y.Pos.Filename),
			cmp.Compare(x.Pos.Offset, y.Pos.Offset),
			cmp.Compare(x.Rule, y.Rule),
		)
	})
	return findings, nil
}
