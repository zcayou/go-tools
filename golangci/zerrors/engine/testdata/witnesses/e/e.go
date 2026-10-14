// Package e raises an error no caller can name.
package e

import "errors"

func rawe() error { return errors.New("e") }

func Raw() error { return rawe() }
