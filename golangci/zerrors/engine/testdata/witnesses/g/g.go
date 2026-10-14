// Package g raises an error no caller can name.
package g

import "errors"

func rawg() error { return errors.New("g") }

func Raw() error { return rawg() }
