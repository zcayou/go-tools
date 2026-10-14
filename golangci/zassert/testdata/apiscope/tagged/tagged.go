//go:build zasserttagged

package tagged

import "errors"

func Do() error { return errors.New("boom") }
