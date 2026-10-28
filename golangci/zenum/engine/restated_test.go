package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/zenum/engine"
)

var _ = Describe("restatements", func() {
	related := engine.Config{RelatedEnums: []engine.Pair{
		{First: "example.com/restated/b.State", Second: "example.com/restated/a.Status"},
	}}

	// Lone shares Solo's one token, and Weekday restates an enum of another
	// module, which no rule judges: neither is reported.
	It("reports the later of each pair, naming the earlier", func() {
		Expect(analyzeFixture("restated", related)).To(Equal([]string{
			"b/b.go:4:6: restated: example.com/restated/b.Paint",
			"b/b.go:23:6: restated: example.com/restated/b.Tone",
			"b/b.go:42:6: restated: example.com/restated/b.Measure",
		}))
	})

	It("says how the tokens overlap", func() {
		Expect(messages("restated", related)).To(Equal(map[string]string{
			"b/b.go:4:6":  "restated: b.Paint restates the tokens of example.com/restated/a.Color",
			"b/b.go:23:6": "restated: b.Tone restates a subset of the tokens of example.com/restated/a.Shade",
			"b/b.go:42:6": "restated: b.Measure restates and extends the tokens of example.com/restated/a.Size",
		}))
	})

	It("reports a related pair no entry lists", func() {
		Expect(analyzeFixture("restated", engine.Config{})).To(
			ContainElement("b/b.go:71:6: restated: example.com/restated/b.State"))
	})

	DescribeTable("fails on a related pair that suppresses nothing",
		func(pair engine.Pair, expected *engine.ExceptionError) {
			err := analyzeError("restated", engine.Config{RelatedEnums: []engine.Pair{pair}})

			Expect(err).To(MatchError(expected))
		},
		Entry("one that is no enum",
			engine.Pair{First: "example.com/restated/a.Color", Second: "example.com/foreign.Weekday"},
			&engine.ExceptionError{
				Setting: engine.SettingRelatedEnums, Fault: engine.FaultNotEnum,
				Type: "example.com/foreign.Weekday", Related: "example.com/restated/a.Color",
			}),
		Entry("a pair that is no restatement",
			engine.Pair{First: "example.com/restated/a.Solo", Second: "example.com/restated/b.Lone"},
			&engine.ExceptionError{
				Setting: engine.SettingRelatedEnums, Fault: engine.FaultNotRestated,
				Type: "example.com/restated/a.Solo", Related: "example.com/restated/b.Lone",
			}),
	)
})

var _ = Describe("bridges", func() {
	listed := engine.Config{RelatedEnums: []engine.Pair{
		{First: "example.com/bridges/chat.listed", Second: "example.com/bridges/catalog.Presence"},
	}}

	// partial leaves a member unpaired, merged pairs two members with one,
	// and Wire is visible: none of them is reported.
	It("reports a private enum a bridge maps member for member onto another", func() {
		Expect(analyzeFixture("bridges", listed)).To(Equal([]string{
			// A switch returning the other's members, one assigning them, a map
			// literal, and a switch over the visible enum returning the copy's.
			"chat/copies.go:7:6: restated: example.com/bridges/chat.presence",
			"chat/copies.go:35:6: restated: example.com/bridges/chat.status",
			"chat/copies.go:63:6: restated: example.com/bridges/chat.mood",
			"chat/copies.go:82:6: restated: example.com/bridges/chat.state",
		}))
	})

	It("names the bridge and the enum to use", func() {
		Expect(messages("bridges", listed)).To(HaveKeyWithValue("chat/copies.go:7:6",
			"restated: presence restates catalog.Presence member for member, as exported maps them; "+
				"use catalog.Presence"))
	})

	It("reports a bridged pair no related-enums entry lists", func() {
		Expect(analyzeFixture("bridges", engine.Config{})).To(
			ContainElement("chat/copies.go:109:6: restated: example.com/bridges/chat.listed"))
	})

	It("fails on a related pair neither tokens nor a bridge restate", func() {
		Expect(analyzeError("bridges", engine.Config{RelatedEnums: []engine.Pair{
			{First: "example.com/bridges/chat.partial", Second: "example.com/bridges/catalog.Presence"},
		}})).To(MatchError(&engine.ExceptionError{
			Setting: engine.SettingRelatedEnums, Fault: engine.FaultNotRestated,
			Type: "example.com/bridges/chat.partial", Related: "example.com/bridges/catalog.Presence",
		}))
	})
})
