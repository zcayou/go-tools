package open

import (
	"fmt"

	"example.com/open/wire"
)

func use(s string, n int, c wire.Code) (wire.ID, wire.Code) {
	if wire.ID(s) == "admin" {
	}
	fmt.Printf("%d\n", c)
	return wire.ID(s), wire.Code(n)
}
