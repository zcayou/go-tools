package fixturescopes_test

import "testing"

type record struct{ shadowed int }

func (r record) get() int { return r.shadowed }

type table map[int]int

type box[T any] struct{ v T }

func (b box[shadowed]) get() shadowed { return b.v }

func pick[shadowed any](v shadowed) shadowed { return v }

func TestReader(t *testing.T) {
	{
		shadowed := 1
		_ = shadowed
	}
	{
		type shadowed int
		_ = shadowed(1)
	}
	_ = func(shadowed int) int { return shadowed }(1)
	var anything any = 1
	switch shadowed := anything.(type) {
	case int:
		_ = shadowed
	}
	for shadowed := range 3 {
		_ = shadowed
	}
shadowed:
	for {
		break shadowed
	}
	_ = record{shadowed: 1}.get()
	_ = box[int]{v: 1}.get()
	_ = pick(1)
	_ = map[int]int{mapKey: 1}
	_ = []map[int]int{{elidedKey: 1}}
	_ = table{namedKey: 1}
}
