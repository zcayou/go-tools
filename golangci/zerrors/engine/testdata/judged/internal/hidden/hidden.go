// Package hidden is internal, and so never an API package.
package hidden

import "errors"

func Raw() error { return errors.New("raw") }
