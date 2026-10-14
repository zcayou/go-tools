// Package a raises an error no caller can name.
package a

import "errors"

func rawa() error { return errors.New("a") }

func Raw() error { return rawa() }
