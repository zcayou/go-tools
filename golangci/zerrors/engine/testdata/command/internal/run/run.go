// Package run is all a command does, which nothing outside the module can call.
package run

import (
	"errors"
	"strings"
)

// Run returns a raw error, which no caller outside the module ever receives.
func Run() error {
	if err := step(); err != nil && strings.Contains(err.Error(), "retry") {
		return errors.New("retried")
	}
	return nil
}

func step() error { return nil }
