package main

import "leak/lib"

func main() {
	_ = lib.Done(lib.Stop, lib.ModeOn, lib.High)
	_ = lib.Describe()
}
