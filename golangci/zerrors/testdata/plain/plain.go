// Package plain returns an error no caller can identify, and decides on one's
// text.
package plain

import (
	"errors"
	"strings"
)

func Open() error {
	return errors.New("closed")
}

func Closed(err error) bool {
	return strings.Contains(err.Error(), "closed")
}
