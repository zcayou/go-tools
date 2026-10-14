// Package f raises an error no caller can name.
package f

import "errors"

func rawf() error { return errors.New("f") }

func Raw() error { return rawf() }
