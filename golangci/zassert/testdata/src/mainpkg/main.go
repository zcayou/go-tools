package main

import "errors"

func do() error { return errors.New("boom") }

func main() { _ = do() }
