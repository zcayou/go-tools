// Package h raises an error no caller can name.
package h

import "errors"

func rawh() error { return errors.New("h") }

func Raw() error { return rawh() }
