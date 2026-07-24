package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

var _ = Describe("evidence", func() {
	It("derives evidence from a boxed constructor's signature", func() {
		pkgs := typecheckPackages(`package box

type Widget struct{}

func (Widget) Spin() {}

func NewWidget() *Widget { return &Widget{} }

func sink(v any) {}

func run() { sink(NewWidget) }
`)
		ev := engine.NewEvidence(buildSSA(pkgs), engine.NewInstantiations(pkgs))

		Expect(ev.Materialized(namedMethod(pkgs[0], "Widget", "Spin"))).To(BeTrue())
	})

	It("grants a type only where a non-skip position reaches it", func() {
		pkgs := typecheckPackages(`package skipsem

type Inner struct{}

type Outer struct{ In Inner }

func sink(v any) {}

func run() { sink(Outer{}) }
`)
		ev := engine.NewEvidence(buildSSA(pkgs), engine.NewInstantiations(pkgs))

		// Outer's underlying struct sits in skip position: it is traversed — Inner,
		// its field, is granted through it — but granted nothing itself, because
		// reflection cannot obtain the underlying as such.
		outer := namedType(pkgs[0], "Outer")
		Expect(ev.Granted(outer)).To(BeTrue())
		Expect(ev.Granted(namedType(pkgs[0], "Inner"))).To(BeTrue())
		Expect(ev.Granted(outer.Underlying())).To(BeFalse())
	})

	It("grants a type reached in both skip and non-skip positions, whichever came first", func() {
		pkgs := typecheckPackages(`package upgrade

type Solo struct{ A int }

type Box struct{ F struct{ A int } }

func sink(v any) {}

func run() {
	sink(Solo{})
	sink(Box{})
}
`)
		ev := engine.NewEvidence(buildSSA(pkgs), engine.NewInstantiations(pkgs))

		// struct{A int} is Solo's underlying — skip — and Box's field — non-skip.
		// The sweep's visit order is not deterministic, so the grant holds only if
		// a non-skip revisit upgrades the memo.
		Expect(ev.Granted(namedType(pkgs[0], "Solo").Underlying())).To(BeTrue())
	})

	It("records a generic origin when an instantiation enters the closure", func() {
		pkgs := typecheckPackages(`package origin

type Runtime[C any] struct{ config C }

func NewRuntime[C any](config C) func() *Runtime[C] {
	return func() *Runtime[C] { return &Runtime[C]{config: config} }
}

func (r *Runtime[C]) Pull() {}

type Ghost[C any] struct{}

func (g *Ghost[C]) Drift() {}

func sink(v any) {}

func run() { sink(NewRuntime("configured")) }
`)
		ev := engine.NewEvidence(buildSSA(pkgs), engine.NewInstantiations(pkgs))

		// The closure holds *Runtime[string]; the declared method's receiver
		// is the origin Runtime[C], and only the recorded origin lets them meet.
		Expect(ev.Materialized(namedMethod(pkgs[0], "Runtime", "Pull"))).To(BeTrue())
		Expect(ev.Materialized(namedMethod(pkgs[0], "Ghost", "Drift"))).To(BeFalse())
	})

	It("derives both the key and the element of a boxed map", func() {
		pkgs := typecheckPackages(`package kv

type Key struct{}

func (Key) Hash() {}

type Val struct{}

func (Val) Get() {}

func sink(v any) {}

func run() { sink(map[Key]Val{}) }
`)
		ev := engine.NewEvidence(buildSSA(pkgs), engine.NewInstantiations(pkgs))

		Expect(ev.Materialized(namedMethod(pkgs[0], "Key", "Hash"))).To(BeTrue())
		Expect(ev.Materialized(namedMethod(pkgs[0], "Val", "Get"))).To(BeTrue())
	})

	It("seeds the closure from resolved reflect.TypeFor instantiations", func() {
		// The first source stands in for the real reflect by path; the second
		// resolves TypeFor's argument only transitively, through the vector
		// of the generic helper that calls it.
		pkgs := typecheckPackages(`package reflect

func TypeFor[T any]() uintptr { return 0 }
`, `package app

import "reflect"

type Secret struct{}

func (Secret) Reveal() {}

type Shadow struct{}

func (Shadow) Reveal() {}

func descriptor[T any]() uintptr { return reflect.TypeFor[T]() }

func run() { _ = descriptor[Secret]() }
`)
		ev := engine.NewEvidence(buildSSA(pkgs), engine.NewInstantiations(pkgs))

		Expect(ev.Materialized(namedMethod(pkgs[1], "Secret", "Reveal"))).To(BeTrue())
		Expect(ev.Materialized(namedMethod(pkgs[1], "Shadow", "Reveal"))).To(BeFalse())
	})

	It("yields each distinct conversion pair once", func() {
		pkgs := typecheckPackages(`package pairs

type Boxer interface{ Box() }

type Widget struct{}

func (Widget) Box() {}

func sink(b Boxer) {}

func run() {
	sink(Widget{})
	sink(Widget{})
}
`)
		ev := engine.NewEvidence(buildSSA(pkgs), engine.NewInstantiations(pkgs))

		var rendered []string
		for operand, iface := range ev.Conversions {
			rendered = append(rendered, operand.String()+" => "+iface.String())
		}
		Expect(rendered).To(Equal([]string{"pairs.Widget => pairs.Boxer"}))
	})
})
