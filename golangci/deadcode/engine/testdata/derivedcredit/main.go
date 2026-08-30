package main

// Src is a sealed contract whose marker is unexported. read holds its only
// selection; nothing in the program ever converts a value to Src.
type Src interface{ src() string }

func read(s Src) string { return s.src() }

// Named is the control: same shape, but the method is exported, which is what
// RTA keeps alive off a derived type. Its credit must not change.
type Named interface{ Label() string }

func label(n Named) string { return n.Label() }

// Emitter gives Holder its one conversion, which seeds the closure.
type Emitter interface{ Emit() string }

type Holder struct{}

func (Holder) Emit() string { return "held" }

// Make is exported, so its result type is derived into the closure off Holder.
func (Holder) Make() ViaMethod { return ViaMethod{} }

type ViaMethod struct{}

// src is credited by nothing: reflection reaching a Holder reaches no
// unexported method on what Make returns.
func (ViaMethod) src() string { return "method" }

// Label is credited by the derivation, because reflection does reach it.
func (ViaMethod) Label() string { return "labelled" }

type ViaFunc struct{}

func (ViaFunc) src() string { return "func" }

// makeFunc is a free function, whose result the closure never traverses.
func makeFunc() ViaFunc { return ViaFunc{} }

func main() {
	var emitter Emitter = Holder{}
	_ = emitter.Emit()
	_ = Holder{}.Make()
	_ = makeFunc()
}
