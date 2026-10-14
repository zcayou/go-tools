// Package contract declares the base every error type carries.
package contract

import "example.com/base/codes"

// Classification is the base.
type Classification struct{ Code codes.Code }

func (c Classification) Error() string { return "classified" }
