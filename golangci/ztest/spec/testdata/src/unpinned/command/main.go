package main

type Failure struct{}

func (Failure) Error() string { return "failure" }

func main() {}
