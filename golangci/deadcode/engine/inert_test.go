package engine_test

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

var _ = Describe("inert-field", func() {
	inert := func(echoes ...string) engine.Config {
		return engine.Config{
			Dir: fixtureDir("inertfield"), Tests: true, API: []string{"./..."},
			Rules: []engine.Rule{engine.RuleInertField}, EchoFunctions: echoes,
		}
	}

	It("reports an exported field of a struct callers hand in that no live module code reads", func(ctx SpecContext) {
		// Read nowhere, only by String, only by a test, only by a function nothing
		// live calls, only by an echo function, and only written; a field of a struct
		// reached through an exported field, one promoted from a struct embedded
		// by value, and an interface method's result the caller authors. A field read
		// in a closure, through a field, or in the whole embedded value promoting it,
		// a deprecated one, the fields of a printed struct, and what a handed struct
		// holds, the caller reading it, are not reported.
		Expect(inertFields(ctx, inert("inertfield/net.Options.Fingerprint"))).To(Equal([]string{
			"net/net.go:12:2: inert-field: Options.Retries",
			"net/net.go:13:2: inert-field: Options.Label",
			"net/net.go:14:2: inert-field: Options.Debug",
			"net/net.go:19:2: inert-field: Options.Hint",
			"net/net.go:21:2: inert-field: Options.Key",
			"net/net.go:22:2: inert-field: Options.Written",
			"net/net.go:37:21: inert-field: Options.Max",
			"net/net.go:31:2: inert-field: Inner.Unused",
			"net/net.go:56:20: inert-field: Reply.Accepted",
		}))
	})

	It("counts what an unlisted function reads", func(ctx SpecContext) {
		Expect(inertFields(ctx, inert())).NotTo(ContainElement("net/net.go:21:2: inert-field: Options.Key"))
	})

	It("says who sets the field", func(ctx SpecContext) {
		found, err := engine.Analyze(ctx, inert("inertfield/net.Options.Fingerprint"))

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(ContainElement(HaveField("Detail",
			"Reply.Accepted is set by callers of inertfield/net.Register and read by nothing; act on it or remove it")))
	})

	It("requires api", func(ctx SpecContext) {
		_, err := engine.Analyze(ctx, engine.Config{Dir: fixtureDir("inertfield"), Rules: []engine.Rule{engine.RuleInertField}})

		Expect(err).To(MatchError(&engine.ConfigError{Rule: engine.RuleInertField, Problem: engine.ProblemRequiresAPI}))
	})

	It("rejects echo-functions while it does not run", func(ctx SpecContext) {
		_, err := engine.Analyze(ctx, engine.Config{
			Dir: fixtureDir("inertfield"), API: []string{"./..."}, EchoFunctions: []string{"inertfield/net.Dial"},
		})

		Expect(err).To(MatchError(&engine.ConfigError{Setting: engine.SettingEchoFunctions, Problem: engine.ProblemServesNoRule}))
	})

	DescribeTable("fails on an echo-functions entry that exempts nothing",
		func(ctx SpecContext, entry string, problem engine.Problem) {
			_, err := engine.Analyze(ctx, inert(entry))

			Expect(err).To(beConfigError(engine.SettingEchoFunctions, problem, entry))
		},
		Entry("one naming no function", "inertfield/net.Nope", engine.ProblemNamesNoFunction),
		Entry("one reading no field another read would not", "inertfield/net.Register", engine.ProblemChangesNothing),
	)
})

// inertFields runs the engine over the inertfield fixture and renders
// inert-field's findings.
func inertFields(ctx SpecContext, cfg engine.Config) []string {
	GinkgoHelper()

	found, err := engine.Analyze(ctx, cfg)
	Expect(err).NotTo(HaveOccurred())
	var reported []string
	for _, finding := range found {
		if finding.Rule != engine.RuleInertField {
			continue
		}
		rel, err := filepath.Rel(cfg.Dir, finding.Pos.Filename)
		Expect(err).NotTo(HaveOccurred())
		reported = append(reported, fmt.Sprintf("%s:%d:%d: %s: %s",
			rel, finding.Pos.Line, finding.Pos.Column, finding.Category(), finding.Name))
	}
	return reported
}
