package api_test

import (
	"fmt"
	"io"
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

func TestSink(t *testing.T) {
	var w io.Writer = api.NewSink()
	fmt.Fprint(w, "x")
}

func TestLabel(t *testing.T) {
	if fmt.Sprint(api.Label("x")) != "x" {
		t.Fatal("label")
	}
}
