// Package c raises an error no caller can name.
package c

import "errors"

func rawc() error { return errors.New("c") }

func Raw() error { return rawc() }
