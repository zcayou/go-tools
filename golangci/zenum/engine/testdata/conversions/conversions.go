package conversions

import "example.com/conversions/kinds"

func convert(n int, k kinds.Kind) []kinds.Kind {
	return []kinds.Kind{
		kinds.Kind(n),
		kinds.Kind(99),
		kinds.Kind(1),
		kinds.Kind(0),
		kinds.Kind(k),
		kinds.Kind(kinds.KindB),
	}
}
