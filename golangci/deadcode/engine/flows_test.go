package engine_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zcayou/go-tools/golangci/deadcode/engine"
)

var _ = Describe("interfaceFlows", func() {
	It("carries a selection through a conversion into an identical re-declaration", func() {
		pkgs := typecheckPackages(`package flow

type Source interface{ List() []string }

type adapter interface{ List() []string }

type ignored interface{ List() []string }

type impl struct{}

func (impl) List() []string { return nil }

func provide() Source { return impl{} }

func consume(a adapter) []string { return a.List() }

func run() { consume(provide()) }
`)
		prog := buildSSA(pkgs)
		inst := engine.NewInstantiations(pkgs)
		refs := engine.NewMethodReferenceScan(pkgs, engine.NewFileFacts(pkgs), engine.NewEvidence(prog, inst))
		flows := engine.NewInterfaceFlows(prog, inst, refs)

		fset := pkgs[0].Fset
		Expect(flows.Used(engine.MethodKey(fset, interfaceMethod(pkgs[0], "adapter", "List")))).To(BeTrue())
		Expect(flows.Used(engine.MethodKey(fset, interfaceMethod(pkgs[0], "Source", "List")))).To(BeTrue())
		Expect(flows.Used(engine.MethodKey(fset, interfaceMethod(pkgs[0], "ignored", "List")))).To(BeFalse())
	})

	It("carries a selection through a constraint an interface argument satisfies", func() {
		pkgs := typecheckPackages(`package flow

type Source interface{ Names() []string }

type adapter[D any] interface{ Names() []D }

func consume[D any, R adapter[D]](r R) []D { return r.Names() }

func run() {
	var s Source
	_ = consume[string, Source](s)
}
`)
		prog := buildSSA(pkgs)
		inst := engine.NewInstantiations(pkgs)
		refs := engine.NewMethodReferenceScan(pkgs, engine.NewFileFacts(pkgs), engine.NewEvidence(prog, inst))
		flows := engine.NewInterfaceFlows(prog, inst, refs)

		Expect(flows.Used(engine.MethodKey(pkgs[0].Fset, interfaceMethod(pkgs[0], "Source", "Names")))).To(BeTrue())
	})

	It("does not treat a type assertion between interfaces as a flow", func() {
		pkgs := typecheckPackages(`package flow

type Source interface{ List() []string }

type adapter interface{ List() []string }

func peek(s Source) []string {
	if a, ok := s.(adapter); ok {
		return a.List()
	}
	return nil
}

func run() { _ = peek(nil) }
`)
		prog := buildSSA(pkgs)
		inst := engine.NewInstantiations(pkgs)
		refs := engine.NewMethodReferenceScan(pkgs, engine.NewFileFacts(pkgs), engine.NewEvidence(prog, inst))
		flows := engine.NewInterfaceFlows(prog, inst, refs)

		fset := pkgs[0].Fset
		Expect(flows.Used(engine.MethodKey(fset, interfaceMethod(pkgs[0], "adapter", "List")))).To(BeTrue())
		Expect(flows.Used(engine.MethodKey(fset, interfaceMethod(pkgs[0], "Source", "List")))).To(BeFalse())
	})

	It("counts a flow into an interface outside the scan unconditionally", func() {
		pkgs := typecheckPackages(`package dep

type Sink interface{ Flush() error }
`, `package flow

import "dep"

type Source interface{ Flush() error }

func hand(s Source) dep.Sink { return s }

func run() {
	var s Source
	_ = hand(s)
}
`)
		prog := buildSSA(pkgs)
		inst := engine.NewInstantiations(pkgs)
		// The reference scan holds only the analyzed package, so dep.Sink sits
		// outside it and nothing ever selects Flush.
		refs := engine.NewMethodReferenceScan(pkgs[1:], engine.NewFileFacts(pkgs[1:]), engine.NewEvidence(prog, inst))
		flows := engine.NewInterfaceFlows(prog, inst, refs)

		Expect(flows.Used(engine.MethodKey(pkgs[1].Fset, interfaceMethod(pkgs[1], "Source", "Flush")))).To(BeTrue())
	})
})
