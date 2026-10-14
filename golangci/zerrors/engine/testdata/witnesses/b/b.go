// Package b raises an error no caller can name.
package b

import "errors"

func rawb() error { return errors.New("b") }

func Raw() error { return rawb() }
