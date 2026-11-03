package main

import (
	"os"

	"example.com/command/internal/run"
)

func main() {
	if err := run.Run(); err != nil {
		os.Exit(1)
	}
}
