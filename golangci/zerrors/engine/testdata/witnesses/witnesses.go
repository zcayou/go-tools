// Package witnesses returns one of eight unstructured errors, each raised
// in a package of its own, so that which one a finding names depends on how
// witnesses are ordered rather than on the order files were parsed in.
package witnesses

import (
	"example.com/witnesses/a"
	"example.com/witnesses/b"
	"example.com/witnesses/c"
	"example.com/witnesses/d"
	"example.com/witnesses/e"
	"example.com/witnesses/f"
	"example.com/witnesses/g"
	"example.com/witnesses/h"
)

func pick(n int) error {
	switch n {
	case 0:
		return h.Raw()
	case 1:
		return g.Raw()
	case 2:
		return f.Raw()
	case 3:
		return e.Raw()
	case 4:
		return d.Raw()
	case 5:
		return c.Raw()
	case 6:
		return b.Raw()
	}
	return a.Raw()
}

func Pick(n int) error { return pick(n) }
