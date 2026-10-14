package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("visibility", func() {
	// Every enum in the fixture is unexported and declares no name, so unnamed
	// reports exactly the visible ones. Those left out are private, hidden behind
	// an unexported field, unexposed, a parameter of a function type, secret
	// behind an unexported field of a converted struct, only compared, and handed
	// only to sinks that never render it. boxed is converted only in a generic
	// type's method an interface dispatches to.
	It("finds each exposure and each carriage", func() {
		Expect(analyzeFixture("visibility", engine.Config{})).To(Equal([]string{
			"conversion.go:8:6: unnamed: example.com/visibility.logged",
			"conversion.go:14:6: unnamed: example.com/visibility.passed",
			"conversion.go:22:6: unnamed: example.com/visibility.pointed",
			"conversion.go:31:6: unnamed: example.com/visibility.sliced",
			"conversion.go:37:6: unnamed: example.com/visibility.arrayed",
			"conversion.go:43:6: unnamed: example.com/visibility.keyed",
			"conversion.go:47:6: unnamed: example.com/visibility.valued",
			"conversion.go:53:6: unnamed: example.com/visibility.channeled",
			"conversion.go:59:6: unnamed: example.com/visibility.wrapped",
			"conversion.go:74:6: unnamed: example.com/visibility.embedded",
			"conversion.go:84:6: unnamed: example.com/visibility.initialized",
			"conversion.go:90:6: unnamed: example.com/visibility.instantiated",
			"conversion.go:98:6: unnamed: example.com/visibility.closed",
			"dispatch.go:7:6: unnamed: example.com/visibility.boxed",
			"exposure.go:14:6: unnamed: example.com/visibility.field",
			"exposure.go:29:6: unnamed: example.com/visibility.variable",
			"exposure.go:36:6: unnamed: example.com/visibility.member",
			"exposure.go:41:6: unnamed: example.com/visibility.param",
			"exposure.go:48:6: unnamed: example.com/visibility.result",
			"exposure.go:55:6: unnamed: example.com/visibility.method",
			"exposure.go:65:6: unnamed: example.com/visibility.level",
			"exposure.go:86:6: unnamed: example.com/visibility.sourced",
			"exposure.go:95:6: unnamed: example.com/visibility.code",
			"exposure.go:102:6: unnamed: example.com/visibility.slot",
			"sinks.go:57:6: unnamed: example.com/visibility.escaped",
		}))
	})

	It("leaves a private enum's unreadable String unjudged", func() {
		Expect(analyzeFixture("forms", engine.Config{})).NotTo(ContainElement(ContainSubstring("forms.quiet")))
	})
})
