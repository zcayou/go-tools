// Package d raises an error no caller can name.
package d

import "errors"

func rawd() error { return errors.New("d") }

func Raw() error { return rawd() }
