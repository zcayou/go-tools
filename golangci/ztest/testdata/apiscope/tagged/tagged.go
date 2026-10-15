//go:build ztesttagged

package tagged

import "errors"

func Do() error { return errors.New("boom") }
