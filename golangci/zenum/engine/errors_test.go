package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("ExceptionError", func() {
	DescribeTable("renders the entry and why it suppresses nothing",
		func(err *engine.ExceptionError, expected string) {
			Expect(err.Error()).To(Equal(expected))
		},
		Entry("a type that is no enum",
			&engine.ExceptionError{
				Setting: engine.SettingOpenEnums, Fault: engine.FaultNotEnum, Type: "example.com/m.ID",
			},
			"open-enums: example.com/m.ID is not an enum of this module"),
		Entry("a zero default with no member at zero",
			&engine.ExceptionError{
				Setting: engine.SettingZeroDefaults, Fault: engine.FaultNoZeroMember, Type: "example.com/m.Kind",
			},
			"zero-defaults: example.com/m.Kind has no member at zero"),
		Entry("a related pair that is no restatement",
			&engine.ExceptionError{
				Setting: engine.SettingRelatedEnums, Fault: engine.FaultNotRestated,
				Type: "example.com/m.A", Related: "example.com/m.B",
			},
			"related-enums: example.com/m.A and example.com/m.B do not restate each other's tokens"),
		Entry("an ordered enum no judged ordering needs",
			&engine.ExceptionError{
				Setting: engine.SettingOrderedEnums, Fault: engine.FaultNotOrdered, Type: "example.com/m.Level",
			},
			"ordered-enums: nothing outside example.com/m.Level's package orders it, "+
				"and nothing inside bounds it by its highest member"),
		Entry("no setting or fault at all",
			&engine.ExceptionError{Type: "example.com/m.Kind"},
			"Setting(0): example.com/m.Kind suppresses nothing"),
	)

	DescribeTable("names each setting by its key",
		func(setting engine.Setting, expected string) {
			Expect(setting.String()).To(Equal(expected))
		},
		Entry(nil, engine.SettingOpenEnums, "open-enums"),
		Entry(nil, engine.SettingZeroDefaults, "zero-defaults"),
		Entry(nil, engine.SettingRelatedEnums, "related-enums"),
		Entry(nil, engine.SettingOrderedEnums, "ordered-enums"),
		Entry(nil, engine.SettingTests, "tests"),
		Entry(nil, engine.SettingOrderedInstantiations, "ordered-instantiations"),
	)

	DescribeTable("names each fault",
		func(fault engine.Fault, expected string) {
			Expect(fault.String()).To(Equal(expected))
		},
		Entry(nil, engine.FaultNotEnum, "not-a-enum"),
		Entry(nil, engine.FaultNoZeroMember, "no-zero-member"),
		Entry(nil, engine.FaultNotRestated, "not-restated"),
		Entry(nil, engine.FaultNotOrdered, "not-ordered"),
	)
})
