package main

import (
	"testconsumers/api"
	"testconsumers/internal/machine"
)

type logger struct{}

func (logger) Observe(event string) { _ = event }

func main() {
	m := machine.Open()
	_ = m.Registrar()
	api.Notify(logger{}, "start")
}
