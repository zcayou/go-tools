package main

import "forwarders/facade"

func main() {
	_ = facade.Encode("a")
	_ = facade.Wrap[int]("b")
	_ = facade.Forward(1)
	_ = facade.Join("/", "c", "d")
}
