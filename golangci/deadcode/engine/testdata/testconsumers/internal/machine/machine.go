// Package machine is production code outside the declared surface.
package machine

import "testconsumers/api"

// Machine collects registrations.
type Machine struct{ names []string }

// Open builds a Machine.
func Open() *Machine { return &Machine{} }

// Registrar hands out the API seam, converting the unexported implementation.
func (m *Machine) Registrar() api.Registrar { return registrar{machine: m} }

type registrar struct{ machine *Machine }

func (r registrar) Append(name string) error {
	r.machine.names = append(r.machine.names, name)
	return nil
}

// Debug is reached only from the test, and is on no declared surface.
func Debug() string { return debugHelper() }

func debugHelper() string { return "debug" }

// Probe observes for the test alone. A consumer cannot name it, so the test
// handing one to api.Notify stands in for no consumer.
type Probe struct{}

func (Probe) Observe(event string) { _ = event }
