package machine_test

import (
	"testing"

	"testconsumers/api"
	"testconsumers/internal/machine"
)

func TestRegistrar(t *testing.T) {
	if err := machine.Open().Registrar().Append("x"); err != nil {
		t.Fatal(err)
	}
	_ = machine.Debug()
}

func TestProbe(t *testing.T) {
	api.Notify(machine.Probe{}, "probed")
}
