package api_test

import (
	"testing"

	"testconsumers/api"
)

type reader interface{ Read() int }

func TestObserverFunc(t *testing.T) {
	api.Notify(api.ObserverFunc(func(string) {}), "event")
}

func TestMeter(t *testing.T) {
	var r reader = api.Meter{}
	if r.Read() != 1 {
		t.Fatal("read")
	}
}
