package net_test

import (
	"testing"

	"inertfield/net"
)

func TestDebug(t *testing.T) {
	opts := net.Options{Debug: true}
	if !opts.Debug {
		t.Fatal("debug")
	}
	net.Dial(opts)
}
