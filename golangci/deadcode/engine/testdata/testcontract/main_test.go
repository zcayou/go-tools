package main

import "testing"

// contract is declared here and nothing selects sample anywhere, so binding
// fixture to it must not launder fixture.sample. A contract a test file declares
// is this analysis's, not a dependency's — every call site it has is one the run
// loads — so it is weighed like any interface the analyzed packages own, and
// both halves of the dead contract are reported together.
type contract interface {
	sample() string
}

type fixture struct{}

func (fixture) sample() string { return "fixture" }

// register converts a fixture to a contract, which is the bind the rule refuses
// rather than never sees.
func register(c contract) contract { return c }

// used is declared here too and TestContracts selects take, so the bind carrier
// carries confers and neither half of that pair is reported.
type used interface {
	take() string
}

type carrier struct{}

func (carrier) take() string { return "carrier" }

func TestContracts(t *testing.T) {
	if register(fixture{}) == nil {
		t.Fatal("nil contract")
	}
	var taker used = carrier{}
	if taker.take() != "carrier" {
		t.Fatal("wrong value")
	}
}
